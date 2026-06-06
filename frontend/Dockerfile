FROM node:20-alpine AS builder
ARG VITE_API_BASE=/api
ARG VITE_ANALYTICS_API=
ARG VITE_KONG_BASE=/api
ENV VITE_API_BASE=$VITE_API_BASE
ENV VITE_ANALYTICS_API=$VITE_ANALYTICS_API
ENV VITE_KONG_BASE=$VITE_KONG_BASE
WORKDIR /app
COPY package.json ./
RUN npm install
COPY . .
RUN npm run build

FROM nginx:alpine
RUN rm -rf /usr/share/nginx/html/*
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY NOTICE.md /usr/share/nginx/html/NOTICE.html
COPY LICENSE.md /usr/share/nginx/html/LICENSE.html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
