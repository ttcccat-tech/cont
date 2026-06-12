FROM openresty/openresty:alpine
# Config and lua scripts baked into image (flattened layout)
COPY lua/cont/ /usr/local/openresty/lualib/cont/
COPY lua/pdk/ /usr/local/openresty/lualib/pdk/
COPY lua/plugins/ /usr/local/openresty/lualib/plugins/
COPY nginx.conf /usr/local/openresty/nginx/conf/nginx.conf
RUN mkdir -p /usr/local/openresty/nginx/logs
EXPOSE 8000
CMD ["nginx", "-g", "daemon off;"]