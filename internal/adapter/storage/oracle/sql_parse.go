package oracle

import (
	"errors"
	"fmt"
	"strings"
)

var (
	sequenceSectionStart = "-- @SECTION: SEQUENCE_CREATION"
	sequenceSectionEnd   = "-- @END_SECTION: SEQUENCE_CREATION"
	typeSectionStart     = "-- @SECTION: TYPE_CREATION"
	typeSectionEnd       = "-- @END_SECTION: TYPE_CREATION"
	packageSpecStart     = "-- @SECTION: PACKAGE_SPECIFICATION"
	packageSpecEnd       = "-- @END_SECTION: PACKAGE_SPECIFICATION"
	packageBodyStart     = "-- @SECTION: PACKAGE_BODY"
	packageBodyEnd       = "-- @END_SECTION: PACKAGE_BODY"
	errNoCodeBlocks      = errors.New("no code blocks found in section")
	errNoSectionMarkers  = errors.New("section markers not found in PL/SQL content")
)

// Extract parses the raw PL/SQL content and splits it into executable blocks.
// It returns slices of strings for sequences, types, package specs, and package bodies.
func Extract(plsqlContent string) (sequences []string, types []string, packageSpecs []string, packageBodies []string, err error) {
	sequence, err1 := extractSequenceBlocks(plsqlContent)
	types, err2 := extractTypeBlocks(plsqlContent)
	packageSpec, err3 := extractPackageSpecBlocks(plsqlContent)
	packageBody, err4 := extractPackageBodyBlocks(plsqlContent)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return nil, nil, nil, nil, fmt.Errorf("error extracting PL/SQL blocks: %v %v %v %v", err1, err2, err3, err4)
	}

	return sequence, types, packageSpec, packageBody, nil
}

// extractSequenceBlocks extracts sequence creation blocks from the PL/SQL content.
func extractSequenceBlocks(plsqlContent string) ([]string, error) {
	var sequences []string
	sections, err := extractSections(plsqlContent, sequenceSectionStart, sequenceSectionEnd)

	if err != nil && errors.Is(err, errNoSectionMarkers) {
		// No types found, return empty slice without error
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	for _, section := range sections {
		sequences = append(sequences, section)
	}

	return sequences, nil
}

// extractTypeBlocks extracts type creation blocks from the PL/SQL content.
func extractTypeBlocks(plsqlContent string) ([]string, error) {
	var types []string
	sections, err := extractSections(plsqlContent, typeSectionStart, typeSectionEnd)

	if err != nil && errors.Is(err, errNoSectionMarkers) {
		// No sequences found, return empty slice without error
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	for _, section := range sections {
		types = append(types, section)
	}

	return types, nil
}

// extractPackageSpecBlocks extracts package specification blocks from the PL/SQL content.
func extractPackageSpecBlocks(plsqlContent string) ([]string, error) {
	var packageSpecs []string
	sections, err := extractSections(plsqlContent, packageSpecStart, packageSpecEnd)

	if err != nil && errors.Is(err, errNoCodeBlocks) {
		// No package specifications found, return empty slice without error
		return nil, nil
	}

	for _, section := range sections {
		packageSpecs = append(packageSpecs, section)
	}
	return packageSpecs, err
}

// extractPackageBodyBlocks extracts package body blocks from the PL/SQL content
func extractPackageBodyBlocks(plsqlContent string) ([]string, error) {
	var packageBodies []string
	sections, err := extractSections(plsqlContent, packageBodyStart, packageBodyEnd)

	if err != nil && errors.Is(err, errNoSectionMarkers) {
		// No package bodies found, return empty slice without error
		return nil, nil
	}

	for _, section := range sections {
		packageBodies = append(packageBodies, section)
	}
	return packageBodies, err
}

// extractSections extracts code blocks from a specified section in the PL/SQL content.
func extractSections(plsqlContent string, startMarker string, endMarker string) ([]string, error) {
	var sections []string
	startIdx := strings.Index(plsqlContent, startMarker)
	endIdx := strings.Index(plsqlContent, endMarker)

	if startIdx == -1 || endIdx == -1 {
		return nil, errNoSectionMarkers
	}

	if endIdx <= startIdx+len(startMarker) {
		return nil, errNoCodeBlocks
	}

	sectionContent := plsqlContent[startIdx+len(startMarker) : endIdx]
	codeBlocks := strings.Split(sectionContent, "\n/\n")

	for _, block := range codeBlocks {
		trimmedBlock := strings.TrimSpace(block)
		trimmedBlock = strings.TrimSuffix(trimmedBlock, "/")
		if trimmedBlock != "" {
			sections = append(sections, trimmedBlock)
		}
	}

	if len(sections) == 0 {
		return nil, errNoCodeBlocks
	}

	return sections, nil
}
