FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/log-transformer .
EXPOSE 8888
CMD ["./log-transformer"]