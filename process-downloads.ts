/**
 * Process downloaded Google Photos album zips
 * 
 * This script:
 * 1. Extracts zip files from timeliner_repo/downloaded_albums/ (cached in extracted_albums/)
 * 2. Processes images (resize + convert to webp)
 * 3. Organizes them into timeliner_repo/processed/downloaded/
 * 4. Merges with existing index.json
 * 
 * Usage:
 *   npx tsx process-downloads.ts              # Normal run (skip already processed)
 *   npx tsx process-downloads.ts --force      # Force reprocess all images
 *   npx tsx process-downloads.ts --extract    # Force re-extract zips
 */

import * as fs from 'fs';
import * as path from 'path';
import AdmZip from 'adm-zip';
import sharp from 'sharp';

const TIMELINER_REPO = './timeliner_repo';
const DOWNLOADS_DIR = path.join(TIMELINER_REPO, 'downloaded_albums');
const PROCESSED_DIR = path.join(TIMELINER_REPO, 'processed', 'downloaded');
const EXTRACTED_DIR = path.join(TIMELINER_REPO, 'extracted_albums');
const INDEX_FILE = './index.json';

// Supported image extensions
const IMAGE_EXTENSIONS = new Set(['.jpg', '.jpeg', '.png', '.heic', '.webp', '.gif', '.tiff', '.bmp']);

// Parse command line arguments
const args = process.argv.slice(2);
const FORCE_REPROCESS = args.includes('--force') || args.includes('-f');
const FORCE_EXTRACT = args.includes('--extract') || args.includes('-e');

interface PhotoIndex {
  [albumName: string]: string[];
}

async function main() {
  console.log('🚀 Starting download processing...');
  
  if (FORCE_REPROCESS) {
    console.log('🔄 Force reprocess mode: will reprocess all images');
  }
  if (FORCE_EXTRACT) {
    console.log('📦 Force extract mode: will re-extract all zips');
  }

  // Ensure directories exist
  fs.mkdirSync(PROCESSED_DIR, { recursive: true });
  fs.mkdirSync(EXTRACTED_DIR, { recursive: true });

  // Load existing index.json (but we'll rebuild the downloaded albums section)
  let existingIndex: PhotoIndex = {};
  if (fs.existsSync(INDEX_FILE)) {
    try {
      existingIndex = JSON.parse(fs.readFileSync(INDEX_FILE, 'utf-8'));
      console.log(`📖 Loaded existing index with ${Object.keys(existingIndex).length} albums`);
    } catch (err) {
      console.warn('⚠️ Could not parse existing index.json, starting fresh');
    }
  }

  // Step 1: Extract zip files (if not already extracted)
  if (fs.existsSync(DOWNLOADS_DIR)) {
    const zipFiles = fs.readdirSync(DOWNLOADS_DIR)
      .filter(f => f.endsWith('.zip'))
      .map(f => path.join(DOWNLOADS_DIR, f));

    if (zipFiles.length > 0) {
      console.log(`\n📦 Found ${zipFiles.length} zip file(s)`);
      
      for (const zipFile of zipFiles) {
        await extractZipFile(zipFile);
      }
    }
  }

  // Step 2: Process extracted albums
  if (!fs.existsSync(EXTRACTED_DIR)) {
    console.log(`📁 No extracted albums found at ${EXTRACTED_DIR}`);
    return;
  }

  const albumFolders = fs.readdirSync(EXTRACTED_DIR)
    .filter(f => fs.statSync(path.join(EXTRACTED_DIR, f)).isDirectory());

  if (albumFolders.length === 0) {
    console.log('📭 No extracted albums found to process');
    return;
  }

  console.log(`\n🖼️  Processing ${albumFolders.length} album(s) from extracted files...`);

  const newPhotos: PhotoIndex = {};

  for (const albumFolder of albumFolders) {
    const albumPath = path.join(EXTRACTED_DIR, albumFolder);
    console.log(`\n📂 Processing album: ${albumFolder}`);
    
    try {
      const photos = await processAlbumFolder(albumPath, albumFolder);
      if (photos.length > 0) {
        newPhotos[albumFolder] = photos;
      }
    } catch (err) {
      console.error(`❌ Error processing ${albumFolder}:`, err);
    }
  }

  // Merge with existing index
  const mergedIndex: PhotoIndex = { ...existingIndex };
  
  for (const [albumName, photos] of Object.entries(newPhotos)) {
    if (!mergedIndex[albumName]) {
      mergedIndex[albumName] = [];
    }
    // Add only new photos (avoid duplicates)
    const existingSet = new Set(mergedIndex[albumName]);
    for (const photo of photos) {
      if (!existingSet.has(photo)) {
        mergedIndex[albumName].push(photo);
      }
    }
  }

  // Write updated index.json
  console.log('\n💾 Writing updated index.json...');
  fs.writeFileSync(INDEX_FILE, JSON.stringify(mergedIndex, null, 2));

  // Summary
  const totalAlbums = Object.keys(mergedIndex).length;
  const totalPhotos = Object.values(mergedIndex).reduce((sum, arr) => sum + arr.length, 0);
  const newAlbums = Object.keys(newPhotos).length;
  const newPhotoCount = Object.values(newPhotos).reduce((sum, arr) => sum + arr.length, 0);

  console.log('\n✅ Processing complete!');
  console.log(`   📚 Total albums: ${totalAlbums}`);
  console.log(`   🖼️  Total photos: ${totalPhotos}`);
  console.log(`   🆕 New albums added: ${newAlbums}`);
  console.log(`   🆕 New photos added: ${newPhotoCount}`);
}

async function extractZipFile(zipPath: string): Promise<void> {
  const zipName = path.basename(zipPath, '.zip');
  const zip = new AdmZip(zipPath);
  const entries = zip.getEntries();
  
  // Get the album folder name from the first entry
  let albumName = '';
  for (const entry of entries) {
    const pathParts = entry.entryName.split('/');
    if (pathParts[0]) {
      albumName = pathParts[0];
      break;
    }
  }
  
  if (!albumName) {
    albumName = zipName;
  }
  
  const extractPath = path.join(EXTRACTED_DIR, albumName);
  
  // Check if already extracted (and not forcing re-extract)
  if (fs.existsSync(extractPath) && !FORCE_EXTRACT) {
    console.log(`   ⏭️  Already extracted: ${albumName}`);
    return;
  }
  
  console.log(`   📤 Extracting: ${path.basename(zipPath)} -> ${albumName}`);
  
  // Extract only image files
  let extracted = 0;
  for (const entry of entries) {
    if (entry.isDirectory) continue;
    
    const ext = path.extname(entry.entryName).toLowerCase();
    if (!IMAGE_EXTENSIONS.has(ext)) continue;
    
    const fileName = path.basename(entry.entryName);
    const outputPath = path.join(extractPath, fileName);
    
    // Ensure directory exists
    fs.mkdirSync(extractPath, { recursive: true });
    
    // Extract file
    fs.writeFileSync(outputPath, entry.getData());
    extracted++;
  }
  
  console.log(`   ✓ Extracted ${extracted} images`);
}

async function processAlbumFolder(albumPath: string, albumName: string): Promise<string[]> {
  const files = fs.readdirSync(albumPath);
  const photos: string[] = [];
  let processed = 0;
  let skipped = 0;

  for (const fileName of files) {
    const ext = path.extname(fileName).toLowerCase();
    
    // Skip non-image files
    if (!IMAGE_EXTENSIONS.has(ext)) {
      continue;
    }

    const inputPath = path.join(albumPath, fileName);
    
    // Generate output path
    const outputFileName = `${path.basename(fileName, ext)}.webp`;
    const albumDir = path.join(PROCESSED_DIR, sanitizeFilename(albumName));
    const outputPath = path.join(albumDir, outputFileName);

    // Skip if already processed (unless force mode)
    if (fs.existsSync(outputPath) && !FORCE_REPROCESS) {
      photos.push(outputPath);
      skipped++;
      continue;
    }

    try {
      // Read image file
      const imageBuffer = fs.readFileSync(inputPath);

      // Process image: resize and convert to webp
      const processedImage = await processImage(imageBuffer);

      // Ensure directory exists
      fs.mkdirSync(albumDir, { recursive: true });

      // Save processed image
      await sharp(processedImage).toFile(outputPath);

      photos.push(outputPath);
      processed++;

      // Progress indicator
      if (processed % 10 === 0) {
        process.stdout.write(`   Processed ${processed} images...\r`);
      }
    } catch (err) {
      console.error(`   ⚠️ Failed to process ${fileName}:`, err instanceof Error ? err.message : err);
    }
  }

  console.log(`   ✓ Processed ${processed} images, skipped ${skipped}`);
  return photos;
}

async function processImage(buffer: Buffer): Promise<Buffer> {
  const image = sharp(buffer);
  const metadata = await image.metadata();

  const width = metadata.width || 0;
  const height = metadata.height || 0;

  // Resize: max 1920 width for landscape, max 1080 height for portrait
  let resizeOptions: sharp.ResizeOptions;
  
  if (width >= height) {
    // Landscape or square
    resizeOptions = { width: 1920, withoutEnlargement: true };
  } else {
    // Portrait
    resizeOptions = { height: 1080, withoutEnlargement: true };
  }

  return image
    .resize(resizeOptions)
    .webp({ quality: 90 })
    .toBuffer();
}

function sanitizeFilename(name: string): string {
  return name.replace(/[<>:"/\\|?*]/g, '_').trim();
}

// Run the script
main().catch(console.error);

