# Static assets served from the root.
#
# This directory has to exist in git: both apps/web/Dockerfile and
# infra/docker/Dockerfile.web do `COPY --from=builder /app/public ./public`,
# so a build fails outright when the directory is missing from the context.
# Next.js also resolves /favicon.ico from here.