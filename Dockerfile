FROM python:3.10.7-alpine

WORKDIR /usr/src/app

COPY requirements/prod.txt ./requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

CMD [ "python", "-m", "delic" ]