FROM ubuntu:latest

WORKDIR /app

COPY tempmail /app/

RUN apt-get update && apt-get install -y sudo

CMD ["sudo", "./tempmail"]

