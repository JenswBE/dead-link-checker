package report

import (
	"fmt"
	"strings"
)

type SiteReport struct {
	SiteURL string
	Report
}

// NewEmptySiteReport returns a new empty site report.
// All maps are initialized with an empty map.
func NewEmptySiteReport(siteURL string) SiteReport {
	return SiteReport{
		SiteURL: siteURL,
		Statistics: Statistics{
			LinksCountTotal:     0,
			LinksCountByPageURL: map[string]int{},
		},
		BrokenLinksByPageURL: map[string][]BrokenLink{},
	}
}

type Report struct {
	Statistics           Statistics              `json:"Statistics"`
	BrokenLinksByPageURL map[string][]BrokenLink `json:"BrokenLinksByPageURL"`
}

type Statistics struct {
	LinksCountTotal     int            `json:"LinksCountTotal"`
	LinksCountByPageURL map[string]int `json:"LinksCountByPageURL"`
}

type BrokenLink struct {
	// Value of the link, most likely a URL.
	// Can be relative or absolute.
	LinkValue string `json:"LinkValue"`
	// Absolute URL based on the link value.
	// Empty if not a valid URL.
	AbsoluteURL string `json:"AbsoluteURL"`
	// HTML tag, e.g. "img".
	Tag string `json:"Tag"`
	// The text type for this tag is "ATTRIBUTE".
	IsTagTextTypeAttribute bool `json:"IsTagTextTypeAttribute"`
	// The text type for this tag is "CONTENT".
	IsTagTextTypeContent bool `json:"IsTagTextTypeContent"`
	// The text type for this tag is "NONE".
	IsTagTextTypeNone bool `json:"IsTagTextTypeNone"`
	// Key of the text for the tag.
	// E.g. "alt" for an image.
	TagTextKey string `json:"TagTextKey"`
	// Content of the HTML tag or relevant attribute.
	// E.g. text content for anchor, "alt" attribute value for image image, ...
	TagTextValue string `json:"TagTextValue"`
	// HTML attribute of tag, e.g. "src".
	Attribute string `json:"Attribute"`
	// HTTP status code
	StatusCode int `json:"StatusCode"`
	// Human readable status or error description
	StatusDescription string `json:"StatusDescription"`
	// Count of this exact tag/attribute/tag content combo
	CountOnPage int `json:"CountOnPage"`
}

func (l *BrokenLink) String() string {
	if l == nil {
		return ""
	}
	// "link_value=xxx;absolute_url=yyy;tag=zzz;..."
	return fmt.Sprintf(strings.Join([]string{
		"link_value=%s",
		"absolute_url=%s",
		"tag=%s",
		"is_tag_text_type_attribute=%t",
		"is_tag_text_type_content=%t",
		"is_tag_text_type_none=%t",
		"tag_text_key=%s",
		"attribute=%s",
		"status_code=%d",
		"status_description=%s",
		"count_on_page=%s",
	}, ";"),
		l.LinkValue,
		l.AbsoluteURL,
		l.Tag,
		l.IsTagTextTypeAttribute,
		l.IsTagTextTypeContent,
		l.IsTagTextTypeNone,
		l.TagTextKey,
		l.TagTextValue,
		l.Attribute,
		l.StatusCode,
		l.StatusDescription,
		l.CountOnPage,
	)
}
