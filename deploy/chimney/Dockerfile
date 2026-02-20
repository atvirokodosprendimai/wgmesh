FROM scratch
# chimney binary is copied in by the deploy workflow (cross-compiled for the target arch)
COPY chimney /chimney
COPY docs /docs
ENTRYPOINT ["/chimney"]
