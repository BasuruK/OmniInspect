const fs = require('fs');
let file = fs.readFileSync('database_settings.go', 'utf8');

file = file.replace(
    /		if keyMsg, ok := msg\.\(tea\.KeyPressMsg\); ok \{\n			if keyMsg\.String\(\) == "ctrl\+c" \{\n				m\.cancel\(\)\n				return m, tea\.Quit\n			\}\n			var cmd tea\.Cmd\n			m\.dbSettings\.addForm, cmd = m\.dbSettings\.addForm\.Update\(keyMsg\)/,
    `		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			if keyMsg.String() == "ctrl+c" {
				m.cancel()
				return m, tea.Quit
			}
		}
		var cmd tea.Cmd
		m.dbSettings.addForm, cmd = m.dbSettings.addForm.Update(msg)`
);

fs.writeFileSync('database_settings.go', file);
