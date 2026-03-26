.PHONY: web web-dev clean

# Build the single-binary web server (frontend + backend)
web: web/frontend/dist/index.html
	go build -o primitive-web ./web/cmd/web

web/frontend/dist/index.html: web/frontend/src/**/* web/frontend/index.html web/frontend/package.json
	cd web/frontend && npm install && npm run build

# Dev mode: run Go server + Vite dev server with hot reload
web-dev:
	@echo "Start both in separate terminals:"
	@echo "  1) go run ./web/cmd/web"
	@echo "  2) cd web/frontend && npm run dev"

clean:
	rm -f primitive-web
