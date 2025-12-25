package oracle

import (
	"fmt"
	"strings"
)

var (
	sequenceSectionStart = "-- @SECTION: SEQUENCE_CREATION"
	sequenceSectionEnd   = "-- @END_SECTION: SEQUENCE_CREATION"
	packageSpecStart     = "-- @SECTION: PACKAGE_SPECIFICATION"
	packageSpecEnd       = "-- @END_SECTION: PACKAGE_SPECIFICATION"
	packageBodyStart     = "-- @SECTION: PACKAGE_BODY"
	packageBodyEnd       = "-- @END_SECTION: PACKAGE_BODY"
)

// Extract parses the raw PL/SQL content and splits it into executable blocks.
// It returns slices of strings for sequences, package specs, and package bodies.
func Extract(plsqlContent string) (sequences []string, packageSpecs []string, packageBodies []string, err error) {
	sequence, err1 := extractSequenceBlocks(string(plsqlContent))
	packageSpec, err2 := extractPackageSpecBlocks(string(plsqlContent))
	packageBody, err3 := extractPackageBodyBlocks(string(plsqlContent))

	if err1 != nil || err2 != nil || err3 != nil {
		return nil, nil, nil, fmt.Errorf("error extracting PL/SQL blocks: %v %v %v", err1, err2, err3)
	}

	return sequence, packageSpec, packageBody, nil
}

func extractSequenceBlocks(plsqlContent string) ([]string, error) {
	var sequences []string
	sections, err := extractSections(plsqlContent, sequenceSectionStart, sequenceSectionEnd)

	if err != nil && strings.Contains(err.Error(), "WARN:") {
		// No sequences found, return empty slice without error
		return nil, nil
	}

	for _, section := range sections {
		sequences = append(sequences, section)
	}
	return sequences, err
}

func extractPackageSpecBlocks(plsqlContent string) ([]string, error) {
	var packageSpecs []string
	sections, err := extractSections(plsqlContent, packageSpecStart, packageSpecEnd)

	if err != nil && strings.Contains(err.Error(), "WARN:") {
		// No package specifications found, return empty slice without error
		return nil, nil
	}

	for _, section := range sections {
		packageSpecs = append(packageSpecs, section)
	}
	return packageSpecs, err
}

func extractPackageBodyBlocks(plsqlContent string) ([]string, error) {
	var packageBodies []string
	sections, err := extractSections(plsqlContent, packageBodyStart, packageBodyEnd)

	if err != nil && strings.Contains(err.Error(), "WARN:") {
		// No package bodies found, return empty slice without error
		return nil, nil
	}

	for _, section := range sections {
		packageBodies = append(packageBodies, section)
	}
	return packageBodies, err
}

func extractSections(plsqlContent string, startMarker string, endMarker string) ([]string, error) {
	var sections []string
	startIdx := strings.Index(plsqlContent, startMarker)
	endIdx := strings.Index(plsqlContent, endMarker)

	if startIdx == -1 || endIdx == -1 {
		return nil, fmt.Errorf("ERR: section markers not found")
	}

	sectionContent := plsqlContent[startIdx+len(startMarker) : endIdx]
	codeBlocks := strings.Split(sectionContent, "\n/\n")

	for _, block := range codeBlocks {
		trimmedBlock := strings.TrimSpace(block)
		if trimmedBlock != "" {
			sections = append(sections, trimmedBlock)
		}
	}

	if len(sections) == 0 {
		return nil, fmt.Errorf("WARN: no code blocks found in section")
	}

	return sections, nil
}
