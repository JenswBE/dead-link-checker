package check

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/extensions"

	"github.com/JenswBE/dead-link-checker/cmd/config"
	"github.com/JenswBE/dead-link-checker/internal/record"
)

const (
	requestTimeout = 30 * time.Second
)

var ignoredSchemes = []string{
	"data:",
	"ftp:",
	"javascript:",
	"mailto:",
	"tel:",
}

var tags = map[string]tagConfig{
	"a":   {linkAttributes: []string{"href"}},          // Anchors
	"img": {linkAttributes: []string{"src", "srcset"}}, // Images
	"link": { // CSS stylesheets
		linkAttributes: []string{"href"},
		ignoreWhenAttributeMatches: map[string][]string{
			"rel": {"dns-prefetch", "pingback", "preconnect", "profile"},
		},
	},
	"script": {linkAttributes: []string{"src"}},    // JS scripts
	"source": {linkAttributes: []string{"srcset"}}, // Part of <picture>
}

// Run checks the provided site. This call blocks until the whole site is checked.
func Run(siteConfig config.SiteConfig, globalIgnoredLinks []*regexp.Regexp, recorder *record.Recorder) error {
	// Create collector
	ignoredLinks := siteConfig.IgnoredLinks
	ignoredLinks = append(ignoredLinks, globalIgnoredLinks...)
	collector := colly.NewCollector(
		colly.Async(true),
		colly.DisallowedURLFilters(ignoredLinks...),
		colly.IgnoreRobotsTxt(),
		extensions.RandomUserAgent,
	)
	collector.SetRequestTimeout(requestTimeout)

	// Define OnHTML callback
	for linkTag, config := range tags {
		for _, linkAttr := range config.linkAttributes {
			query := fmt.Sprintf("%s[%s]", linkTag, linkAttr)
			collector.OnHTML(query, handleHTML(collector, recorder, linkTag, linkAttr))
		}
	}

	// Define OnRequest callback
	collector.OnRequest(func(r *colly.Request) {
		slog.Debug("Visiting", "url", r.URL)
	})

	// Define OnError callback
	siteURL := siteConfig.URL.String()
	collector.OnError(func(r *colly.Response, err error) {
		// Setup logger
		linkAbsoluteURL := r.Request.Ctx.Get("link_absolute_url")
		logger := slog.With(
			"status_code", r.StatusCode,
			"link_value", r.Request.Ctx.Get("link_value"),
			"link_absolute_url", linkAbsoluteURL,
			"actual_absolute_url", r.Request.URL.String(),
			"page_url", r.Request.Ctx.Get("page_url"),
			"tag", r.Request.Ctx.Get("tag"),
			"attribute", r.Request.Ctx.Get("attribute"),
			"site_url", siteURL,
		)

		// Handle false-positives due to redirection
		var visitedErr *colly.AlreadyVisitedError
		if errors.As(err, &visitedErr) {
			logger.Info("Link already visited, probably due to a redirect. Ignoring ...", "error", err)
			return // Ignore error
		}
		if errors.Is(err, colly.ErrForbiddenURL) && strings.Contains(err.Error(), "redirect") {
			logger.Info("Redirect to ignored link, ignoring ...", "error", err)
			return // Ignore error
		}

		// Report broken link
		report := record.BrokenLink{
			AbsoluteURL: linkAbsoluteURL,
			BrokenLinkDetails: record.BrokenLinkDetails{
				StatusCode:        r.StatusCode,
				StatusDescription: err.Error(),
			},
		}
		recorder.RecordBrokenLink(report)
		logger.Warn("Following link returned error", "error", err)
	})

	// Start initial request
	ctx := colly.NewContext()
	ctx.Put("site_url", siteURL)
	if err := collector.Request(http.MethodGet, siteConfig.URL.String(), nil, ctx, nil); err != nil {
		return fmt.Errorf("failed to start collector for site %s: %w", siteConfig.URL, err)
	}

	// Wait for collector to finish
	collector.Wait()
	return nil
}

func handleHTML(collector *colly.Collector, recorder *record.Recorder, tag, attr string) colly.HTMLCallback {
	return func(e *colly.HTMLElement) {
		// Set context
		e.Request.Ctx.Put("page_url", e.Request.URL.String())
		e.Request.Ctx.Put("tag", tag)
		e.Request.Ctx.Put("attribute", attr)
		linkReport := record.Link{
			PageURL:   e.Request.URL.String(),
			Tag:       tag,
			Attribute: attr,
		}

		// Check if tag should be ignored
		for attr, attrValues := range tags[tag].ignoreWhenAttributeMatches {
			attrValue := strings.TrimSpace(e.Attr(attr))
			if slices.Contains(attrValues, attrValue) {
				slog.Debug("Link ignored because attribute value is in list to ignore", "tag", tag, "attribute", attr, "attribute_value", attrValue)
				return
			}
		}

		// Process attribute
		switch attr {
		case "srcset":
			items := strings.Split(e.Attr("srcset"), ",")
			for _, item := range items {
				// item is e.g. "/images/example4x.jpg 4x"
				itemParts := strings.Split(strings.TrimSpace(item), " ")
				handleLinkValue(collector, recorder, e, itemParts[0], linkReport)
			}
		default:
			handleLinkValue(collector, recorder, e, e.Attr(attr), linkReport)
		}
	}
}

func handleLinkValue(
	collector *colly.Collector,
	recorder *record.Recorder,
	e *colly.HTMLElement,
	linkValue string,
	linkReport record.Link,
) {
	site := e.Request.Ctx.Get("site_url")
	logger := slog.With("site_url", site, "link_value", linkValue)
	if strings.HasPrefix(linkValue, "#") {
		// Skip link as it's a hash link to the current page
		logger.Debug("Link ignored because it is a hash link to the current page", "page_url", e.Request.URL.String())
		return
	}
	if !strings.HasPrefix(e.Request.URL.String(), site) {
		// Skip link as we are already on an external site
		logger.Debug("Link ignored because we are on an external site", "page_url", e.Request.URL.String())
		return
	}
	if hasIgnoredScheme(linkValue) {
		// Skip link as it has an ignored scheme
		logger.Debug("Link ignored because it has an ignored scheme")
		return
	}

	// Extract tag content
	switch e.Name {
	case "a":
		linkReport.TagText = record.NewTagTextContent(e.Text)
	case "img":
		linkReport.TagText = record.NewTagTextAttribute("alt", e.Attr("alt"))
	case "link":
		linkReport.TagText = record.NewTagTextAttribute("rel", e.Attr("rel"))
	default:
		linkReport.TagText = record.NewTagTextNone()
	}

	// Process link
	linkReport.LinkValue = linkValue
	linkReport.AbsoluteURL = e.Request.AbsoluteURL(linkValue)
	recorder.RecordLink(linkReport)

	// Clone context
	ctxClone := colly.NewContext()
	e.Request.Ctx.ForEach(func(k string, v any) any {
		ctxClone.Put(k, v)
		return nil
	})

	// Visit link
	ctxClone.Put("link_value", linkValue)
	ctxClone.Put("link_absolute_url", linkReport.AbsoluteURL)
	err := collector.Request(http.MethodGet, linkReport.AbsoluteURL, nil, ctxClone, nil)
	var visitedErr *colly.AlreadyVisitedError
	if err != nil && !errors.As(err, &visitedErr) && !errors.Is(err, colly.ErrForbiddenURL) {
		slog.Error("Failed to send request. Will mark as broken link.", "error", err, "url", linkReport.AbsoluteURL, "method", http.MethodGet)
		recorder.RecordBrokenLink(record.BrokenLink{
			AbsoluteURL: linkReport.AbsoluteURL,
			BrokenLinkDetails: record.BrokenLinkDetails{
				StatusCode:        0,
				StatusDescription: "Failed to create request: " + err.Error(),
			},
		})
	}
}

func hasIgnoredScheme(linkValue string) bool {
	for _, scheme := range ignoredSchemes {
		if strings.HasPrefix(linkValue, scheme) {
			return true
		}
	}
	return false
}
