FROM node:18-alpine AS builder

WORKDIR /app

RUN npm config set registry https://registry.npmmirror.com

COPY frontend/package*.json ./
RUN npm install \
 && npm install --no-save @rollup/rollup-linux-x64-musl

COPY frontend/ .

RUN npm run build


FROM nginx:alpine

WORKDIR /etc/nginx/conf.d

RUN rm default.conf

COPY docker/nginx.conf ./default.conf

COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 4173

CMD ["nginx", "-g", "daemon off;"]