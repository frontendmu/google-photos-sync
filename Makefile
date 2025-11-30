add:
	~/go/bin/timeliner add-account google_photos/front.end.coders.mu@gmail.com

init:
	~/go/bin/timeliner get-all google_photos/front.end.coders.mu@gmail.com

auth:
	~/go/bin/timeliner reauth google_photos/front.end.coders.mu@gmail.com

sync:
	~/go/bin/timeliner get-latest google_photos/front.end.coders.mu@gmail.com

# Process downloaded album zips from timeliner_repo/downloaded_albums/
process:
	npm run process-downloads

# Force reprocess all images (useful when changing quality settings)
process-force:
	npm run process-downloads:force

# Force re-extract zips and reprocess
process-extract:
	npm run process-downloads:extract

# Generate index.json from timeliner database (legacy)
json:
	go run .

# Full workflow: process downloads then generate combined index
all: process json
