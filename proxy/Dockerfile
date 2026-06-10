FROM openresty/openresty:alpine
# No build step — config and lua scripts mounted at runtime
COPY lua/ /usr/local/openresty/lualib/cont/
COPY nginx.conf /usr/local/openresty/nginx/conf/nginx.conf
EXPOSE 8000
CMD ["nginx", "-g", "daemon off;"]