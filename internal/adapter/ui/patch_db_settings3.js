const fs = require('fs');
let file = fs.readFileSync('database_settings.go', 'utf8');

file = file.replace(
    /			return m, cmd\n		\}\n		return m, nil/,
    `			return m, cmd
		// return m, nil`
);

fs.writeFileSync('database_settings.go', file);
