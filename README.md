# Sync Photos from Front-end Coders Google Albums

Storage for our event albums, consumed by [frontendmu-astro](https://github.com/Front-End-Coders-Mauritius/frontendmu-astro) for frontend.mu

Maintained by [@MrSunshyne](https://github.com/MrSunshyne)

## Quick Start: Adding New Albums

1. Download album from Google Photos (as zip)
2. Place zip in `timeliner_repo/downloaded_albums/`
3. Run:
   ```bash
   make process
   ```

That's it! The `index.json` is updated automatically.

## Commands

| Command | Description |
|---------|-------------|
| `make process` | Process new album zips → updates `index.json` |
| `make process-force` | Reprocess all images (after changing quality settings) |
| `make json` | Regenerate index from legacy timeliner database |

## How It Works

1. **Zips are extracted** to `timeliner_repo/extracted_albums/` (cached)
2. **Images are resized** (1920x1080 max) and converted to webp
3. **Output saved** to `timeliner_repo/processed/downloaded/<album>/`
4. **index.json** is updated with album → photo paths mapping

## Setup (First Time)

```bash
npm install
```

## Legacy Commands (timeliner - deprecated)

These require `timeliner.toml` with OAuth credentials:

| Command | Description |
|---------|-------------|
| `make init` | Get all photos from Google account |
| `make sync` | Get latest photos |
| `make auth` | Re-authenticate |

> Note: Google Photos API access was restricted in March 2025. Manual album downloads are now required.
