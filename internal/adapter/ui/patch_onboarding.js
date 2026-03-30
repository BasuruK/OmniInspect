const fs = require('fs');
let file = fs.readFileSync('onboarding.go', 'utf8');

file = file.replace(
    /	case tea\.KeyPressMsg:\n		return m\.handleOnboardingKey\(msg\)/,
    `	case tea.PasteMsg:
		if !m.onboarding.submitted {
			val := m.onboarding.fieldValue(m.onboarding.step)
			*val += msg.String()
			m.onboarding.errMsg = ""
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleOnboardingKey(msg)`
);

fs.writeFileSync('onboarding.go', file);
