/**
 * Minifies compiled JavaScript files
 * This script runs after TypeScript compilation
 */

const fs = require('fs');
const path = require('path');
const { minify } = require('terser');

const STATIC_JS_DIR = path.join(__dirname, '..', 'static', 'js');

/**
 * Minifies a JavaScript file
 * @param {string} inputFile - Path to input file
 * @param {string} outputFile - Path to output file
 * @returns {Promise<{originalSize: number, minifiedSize: number}>}
 */
async function minifyFile(inputFile, outputFile) {
  const code = fs.readFileSync(inputFile, 'utf8');
  
  const result = await minify(code, {
    compress: {
      dead_code: true,
      drop_debugger: true,
      drop_console: false,
      keep_fargs: false,
      keep_fnames: false,
      passes: 2,
    },
    mangle: {
      toplevel: true,
      properties: false,
    },
    format: {
      comments: false,
    },
  });

  fs.writeFileSync(outputFile, result.code);
  
  const originalSize = Buffer.byteLength(code, 'utf8');
  const minifiedSize = Buffer.byteLength(result.code, 'utf8');
  
  return { originalSize, minifiedSize };
}

/**
 * Removes a file if it exists
 * @param {string} filePath - Path to file
 */
function removeFile(filePath) {
  try {
    fs.unlinkSync(filePath);
  } catch {
    // File doesn't exist, ignore
  }
}

/**
 * Main build function
 */
async function build() {
  console.log('Minifying JavaScript files...\n');

  // Minify analytics.js -> analytics.min.js
  const analyticsInput = path.join(STATIC_JS_DIR, 'analytics.js');
  const analyticsOutput = path.join(STATIC_JS_DIR, 'analytics.min.js');
  
  if (fs.existsSync(analyticsInput)) {
    try {
      const { originalSize, minifiedSize } = await minifyFile(analyticsInput, analyticsOutput);
      const savings = ((originalSize - minifiedSize) / originalSize * 100).toFixed(1);
      console.log(`  ✓ analytics.js → analytics.min.js`);
      console.log(`    ${originalSize} bytes → ${minifiedSize} bytes (${savings}% smaller)`);
      
      // Remove the unminified file
      removeFile(analyticsInput);
    } catch (error) {
      console.error('  ✗ Failed to minify analytics.js:', error.message);
      process.exit(1);
    }
  } else {
    console.log('  ! analytics.js not found, skipping');
  }

  // Minify dashboard.js -> dashboard.min.js
  const dashboardInput = path.join(STATIC_JS_DIR, 'dashboard.js');
  const dashboardOutput = path.join(STATIC_JS_DIR, 'dashboard.min.js');
  
  if (fs.existsSync(dashboardInput)) {
    try {
      const { originalSize, minifiedSize } = await minifyFile(dashboardInput, dashboardOutput);
      const savings = ((originalSize - minifiedSize) / originalSize * 100).toFixed(1);
      console.log(`  ✓ dashboard.js → dashboard.min.js`);
      console.log(`    ${originalSize} bytes → ${minifiedSize} bytes (${savings}% smaller)`);
      
      // Remove the unminified file
      removeFile(dashboardInput);
    } catch (error) {
      console.error('  ✗ Failed to minify dashboard.js:', error.message);
      process.exit(1);
    }
  } else {
    console.log('  ! dashboard.js not found, skipping');
  }

  // talkdom.js lives in static/js/ — it's not compiled from fe_src.
  // Just report its size; it's already small enough to serve as-is.
  const talkdomPath = path.join(STATIC_JS_DIR, 'talkdom.js');

  if (fs.existsSync(talkdomPath)) {
    const size = fs.statSync(talkdomPath).size;
    console.log(`  ✓ talkdom.js (${size} bytes)`);
  } else {
    console.log('  ! talkdom.js not found in static/js');
  }

  console.log('\n✓ Minification complete!');
}

build().catch(error => {
  console.error('Minification failed:', error);
  process.exit(1);
});
