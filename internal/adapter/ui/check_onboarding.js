const fs = require('fs');
let file = fs.readFileSync('onboarding.go', 'utf8');
console.log(file.includes('AddDatabaseForm'));
