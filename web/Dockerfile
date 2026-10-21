FROM node:22-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci
COPY . .
ARG VERSION=dev
ENV VITE_APP_VERSION=$VERSION
RUN npm run build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
ARG VERSION=dev
LABEL org.opencontainers.image.title="hypervane-web" \
      org.opencontainers.image.version="${VERSION}"
EXPOSE 80
