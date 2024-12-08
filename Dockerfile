FROM gcr.io/distroless/static-debian12

COPY goscrape2 /

ENTRYPOINT ["./goscrape2"]

