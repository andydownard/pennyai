FROM node:20-alpine
WORKDIR /app
RUN apk add --no-cache python3 make g++
COPY deck/package.json deck/package-lock.json* ./
RUN npm install --production
COPY deck/server.js deck/deck-engine.js deck/deck.html deck/widget.js ./
COPY deck/lib ./lib/
COPY deck/thriftly-logo.svg deck/penny-avatar.png deck/handwritten-list.png ./
EXPOSE 3000
CMD ["node", "server.js"]
