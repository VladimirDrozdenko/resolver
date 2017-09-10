package resolver

import (
	"errors"
	"log"
	"strings"
	"regexp"
)


//
// Takes text document and resolves all parameters in it according to ResolveOptions.
// It will return a map of (parameter reference) to SsmParameterInfo.
func ExtractParametersFromText(
		service ISsmParameterService,
		input string,
		options ResolveOptions) (map[string]SsmParameterInfo, error) {

	uniqueParameterReferences, err := parseParametersFromTextIntoMap(input, options)
	if err != nil {
		return nil, err
	}

	parametersWithValues, err := getParametersFromSsmParameterStore(service, uniqueParameterReferences)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	if !options.ResolveSecureParameters {

		invalidParameters := []string {}
		for key, value := range parametersWithValues {
			if strings.HasPrefix(key, ssmSecurePrefix) || value.Type == secureStringType {
				invalidParameters = append(invalidParameters, key)
			}
		}

		if len(invalidParameters) > 0 {
			return nil, errors.New("resolving secure parameters is not allowed")
		}
	}

	return parametersWithValues, nil
}

//
// Takes a list of references to SSM parameters, resolves them according to ResolveOptions and
// returns a map of (parameter reference) to SsmParameterInfo.
func ResolveParameterReferenceList(
		service ISsmParameterService,
		parameterReferences []string,
		options ResolveOptions) (map[string]SsmParameterInfo, error) {

	uniqueParameterReferences := dedupSlice(parameterReferences)
	parametersWithValues, err := getParametersFromSsmParameterStore(service, uniqueParameterReferences)
	if err != nil {
		log.Fatal(err)
		return nil, err
	}

	if !options.ResolveSecureParameters {

		invalidParameters := []string {}
		for key, value := range parametersWithValues {
			if strings.HasPrefix(key, ssmSecurePrefix) || value.Type == secureStringType {
				invalidParameters = append(invalidParameters, key)
			}
		}

		if len(invalidParameters) > 0 {
			return nil, errors.New("resolving secure parameters is not allowed")
		}
	}

	return parametersWithValues, nil
}

//
// Takes text document, resolves all parameters in it according to ResolveOptions
// and returns resolved document.
func ResolveParametersInText(
		service ISsmParameterService,
		input string,
		options ResolveOptions) (string, error) {

	resolvedParametersMap, err := ExtractParametersFromText(service, input, options)
	if err != nil || resolvedParametersMap == nil || len(resolvedParametersMap) == 0 {
		return input, err
	}

	for ref, param := range resolvedParametersMap {
		var placeholder = regexp.MustCompile("{{\\s*" + ref + "\\s*}}")
		input = placeholder.ReplaceAllString(input, param.Value)
	}

	return input, nil
}


//
// Reads inputFileName, resolves SSM parameters in it according to ResolveOptions and
// stores resolved document in the outputFileName file.
func ResolveParametersInFile(
		service ISsmParameterService,
		inputFileName string,
		outputFileName string,
		options ResolveOptions) error {

	if len(inputFileName) == 0 {
		return errors.New("input file name is not provided")
	}

	if len(outputFileName) == 0 {
		return errors.New("output file name is not provided")
	}

	errorInFileOrSize := validateFileAndSize(inputFileName)
	if errorInFileOrSize != nil {
		return errorInFileOrSize
	}

	unresolvedText, err := readTextFromFile(inputFileName)
	if err != nil {
		return err
	}

	resolvedParametersMap, err := ExtractParametersFromText(service, unresolvedText, options)
	if err != nil || resolvedParametersMap == nil || len(resolvedParametersMap) == 0 {
		return err
	}

	for ref, param := range resolvedParametersMap {
		var placeholder = regexp.MustCompile("{{\\s*" + ref + "\\s*}}")
		unresolvedText = placeholder.ReplaceAllString(unresolvedText, param.Value)
	}

	err = writeToFile(unresolvedText, outputFileName)
	if err != nil {
		log.Fatal(err)
		return err
	}

	return nil
}

func dedupSlice(slice []string) []string {
	ht := map[string]bool {}

	for _, element := range slice {
		ht[element] = true
	}

	keys := make([]string, len(ht))

	i := 0
	for k := range ht {
		keys[i] = k
		i++
	}

	return keys
}

func parseParametersFromTextIntoMap(text string, options ResolveOptions) ([]string, error) {
	matchedPhrases := parameterPlaceholder.FindAllStringSubmatch(text, -1)
	matchedSecurePhrases := secureParameterPlaceholder.FindAllStringSubmatch(text, -1)

	if !options.ResolveSecureParameters && len(matchedSecurePhrases) > 0 {
		return nil, errors.New("resolving secure parameters is not allowed")
	}

	parameterNamesDeduped := make(map[string]bool)
	for i := 0; i < len(matchedPhrases); i++ {
		parameterNamesDeduped[matchedPhrases[i][1]] = true
	}

	for i := 0; i < len(matchedSecurePhrases); i++ {
		parameterNamesDeduped[matchedSecurePhrases[i][1]] = true
	}

	result := []string {}
	for key, _ := range parameterNamesDeduped {
		result = append(result, key)
	}

	return result, nil
}
