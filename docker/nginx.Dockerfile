FROM alpine:3.20 AS builder

RUN apk add --no-cache \
    build-base \
    ca-certificates \
    git \
    pcre2-dev \
    zlib-dev \
    openssl-dev \
    linux-headers \
    cmake \
    perl

WORKDIR /usr/src

RUN git clone --recurse-submodules -j4 https://github.com/google/ngx_brotli.git && \
    mkdir -p ngx_brotli/deps/brotli/out && \
    cd ngx_brotli/deps/brotli/out && \
    cmake -DCMAKE_BUILD_TYPE=Release \
          -DBUILD_SHARED_LIBS=OFF \
          -DCMAKE_POSITION_INDEPENDENT_CODE=ON .. && \
    make -j$(nproc) brotlienc brotlidec brotlicommon

RUN git clone --depth 1 -b openssl-3.1.5+quic https://github.com/quictls/openssl.git quictls

RUN NGINX_VER="1.26.2" && \
    wget http://nginx.org/download/nginx-${NGINX_VER}.tar.gz && \
    tar -zxvf nginx-${NGINX_VER}.tar.gz && \
    mv nginx-${NGINX_VER} nginx

WORKDIR /usr/src/nginx

RUN ./configure \
    --prefix=/etc/nginx \
    --sbin-path=/usr/sbin/nginx \
    --conf-path=/etc/nginx/nginx.conf \
    --error-log-path=/var/log/nginx/error.log \
    --http-log-path=/var/log/nginx/access.log \
    --pid-path=/var/run/nginx.pid \
    --lock-path=/var/run/nginx.lock \
    --http-client-body-temp-path=/var/cache/nginx/client_temp \
    --http-proxy-temp-path=/var/cache/nginx/proxy_temp \
    --user=nginx \
    --group=nginx \
    --with-compat \
    --with-http_ssl_module \
    --with-http_v2_module \
    --with-http_v3_module \
    --with-openssl=../quictls \
    --add-module=../ngx_brotli && \
    make -j$(nproc) && \
    make install

FROM alpine:3.20

RUN apk add --no-cache \
    pcre2 \
    zlib \
    openssl \
    ca-certificates

RUN addgroup -S nginx
RUN adduser -S -H -G nginx nginx

COPY --from=builder /usr/sbin/nginx /usr/sbin/nginx
COPY --from=builder /etc/nginx /etc/nginx

RUN mkdir -p /var/cache/nginx/client_temp \
             /var/cache/nginx/proxy_temp
RUN chown -R nginx:nginx /var/cache/nginx

EXPOSE 80/tcp 443/tcp 443/udp

CMD ["nginx", "-g", "daemon off;"]