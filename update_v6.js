const fs = require('node:fs');
const path = require('node:path');

const rootDir = '/Users/binhnt/Work/blockchain/vnp-blc/orca/specs/backend-go/crs/v6';

function traverse(dir, callback) {
    fs.readdirSync(dir).forEach(file => {
        const fullPath = path.join(dir, file);
        if (fs.statSync(fullPath).isDirectory()) {
            traverse(fullPath, callback);
        } else if (fullPath.endsWith('.md')) {
            callback(fullPath);
        }
    });
}

traverse(rootDir, (filePath) => {
    let content = fs.readFileSync(filePath, 'utf-8');
    let changed = false;

    // 1. Solution files
    if (filePath.includes('/solutions/') && path.basename(filePath) !== 'README.md') {
        const newHeader = '> ✅ **Đã triển khai.** Toàn bộ code đã được implement và verify (xem task list).';
        // replace any `> 📋 Proposed...` or `> ⏳ Đang triển khai...` with the new header
        content = content.replace(/^>\s*(📋|⏳|✅).*$/m, newHeader);
        
        // replace - [ ] with - [x]
        if (content.includes('- [ ]')) {
            content = content.replace(/- \[ \]/g, '- [x]');
        }
        changed = true;
    }

    // 2. Task files
    if (filePath.includes('/tasks/') && path.basename(filePath) !== 'README.md') {
        // replace - [ ] with - [x]
        if (content.includes('- [ ]')) {
            content = content.replace(/- \[ \]/g, '- [x]');
            changed = true;
        }
    }

    // 3. README.md files in solutions or tasks or feature roots
    if (path.basename(filePath) === 'README.md') {
        if (content.includes('- [ ]')) {
            content = content.replace(/- \[ \]/g, '- [x]');
            changed = true;
        }
    }

    // 4. Root README.md
    if (filePath === path.join(rootDir, 'README.md')) {
        content = content.replace(/^>\s*\*\*Trạng thái:.*$/m, '> **Trạng thái: ✅ Đã hoàn thành (Implemented & Verified).**');
        if (content.includes('- [ ]')) {
            content = content.replace(/- \[ \]/g, '- [x]');
        }
        changed = true;
    }

    if (changed) {
        fs.writeFileSync(filePath, content);
        console.log(`Updated ${filePath}`);
    }
});
