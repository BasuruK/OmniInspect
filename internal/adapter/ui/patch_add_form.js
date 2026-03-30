const fs = require('fs');
let file = fs.readFileSync('add_database_form.go', 'utf8');

file = file.replace(
    /func \(f AddDatabaseForm\) Update\(msg tea\.KeyPressMsg\) \(AddDatabaseForm, tea\.Cmd\) \{/,
    `func (f AddDatabaseForm) Update(msg tea.Msg) (AddDatabaseForm, tea.Cmd) {
	if f.submitted || f.cancelled {
		return f, nil
	}

	switch msg := msg.(type) {
	case tea.PasteMsg:
		if f.cursor < formFieldCount {
			f.fields[f.cursor].Value += msg.String()
			f.errMsg = ""
		}
		return f, nil
	case tea.KeyPressMsg:`
);

file = file.replace(
    /	if f\.submitted \|\| f\.cancelled \{\n		return f, nil\n	\}\n\n	key := msg\.String\(\)/,
    `	key := msg.String()`
);

file = file.replace(
    /	return f, nil\n\}/,
    `	return f, nil
	}
	return f, nil
}`
);

fs.writeFileSync('add_database_form.go', file);
