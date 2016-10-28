package main

import (
	"encoding/csv"
	"fmt"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func buildContentJson(fieldNames []string, fieldValues []string, sortedIndexes []int, fields map[string]Field) string {
	jsonParts := []string{}
	for _, index := range sortedIndexes {
		if len(fieldValues[index]) > 0 {
			jsonPart := ""
			fieldDef := fields[fieldNames[index]]
			escapedValue := escapeQuotes(fieldValues[index])
			if fieldDef.Cardinality == "n" {
				if fieldDef.Datatype == "string" {
					jsonPart = fmt.Sprintf(`"%s":%s`, fieldNames[index], toJsonArrayOfStr(escapedValue))
				} else {
					jsonPart = fmt.Sprintf(`"%s":%s`, fieldNames[index], toJsonArrayOfNum(escapedValue))
				}
			} else {
				jsonPart = fmt.Sprintf(`"%s":"%s"`, fieldNames[index], escapedValue)
			}
			jsonParts = append(jsonParts, jsonPart)
		}
	}
	jsonBody := strings.Join(jsonParts, ",")
	return "{" + jsonBody + "}"
}

func alphabeticalIndexes(fields []string) []int {
	fieldIndexes := make([]FieldIndex, len(fields))
	for index, field := range fields {
		fieldIndexes[index] = FieldIndex{field, index}
	}

	sort.Sort(ByAlphabetical(fieldIndexes))

	sortedIndexes := make([]int, len(fields))
	for i, fieldIndex := range fieldIndexes {
		sortedIndexes[i] = fieldIndex.Index
	}
	return sortedIndexes
}

func processLine(fieldValues []string, fieldNames []string, sortedIndexes []int, fieldDefns map[string]Field) {
	contentJson := buildContentJson(fieldNames, fieldValues, sortedIndexes, fieldDefns)
	contentJsonHash := "sha256:" + sha256Hex([]byte(contentJson))
	itemParts := []string{"append-entry", timestamp(), contentJsonHash}
	itemLine := strings.Join(itemParts, "\t")
	entryParts := []string{"add-item", string(contentJson)}
	entryLine := strings.Join(entryParts, "\t")
	fmt.Println(itemLine)
	fmt.Println(entryLine)
}

func processCSV(fieldsFile, tsvFile io.Reader) {

	fields := readFieldTypes(fieldsFile)

	csvReader := csv.NewReader(tsvFile)
	csvReader.Comma = '\t'
	csvReader.LazyQuotes = true
	//read header
	fieldNames, err := csvReader.Read()
	if err != nil {
		log.Fatal(err)
		return
	}
	sortedIndexes := alphabeticalIndexes(fieldNames)
	for {
		fieldValues, err := csvReader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
			return
		}
		processLine(fieldValues, fieldNames, sortedIndexes, fields)
	}
}

func processYamlFile(fileInfo os.FileInfo, yamlDir string, registerName string) {
	if strings.HasSuffix(fileInfo.Name(), ".yaml") {
		yamlFile, err := os.Open(yamlDir + "/" + fileInfo.Name())
		if err != nil {
			log.Fatal(err)
			return
		}
		defer yamlFile.Close()

		processYaml(yamlFile, registerName)
	}
}

func processYaml(yamlFile io.Reader, registerName string) {
	var contentJson string
	var err error

	switch registerName {
	case "datatype":
		var dt Datatype
		yaml.Unmarshal(streamToBytes(yamlFile), &dt)
		contentJson, err = toJsonStr(dt)
	case "field":
		var f Field
		yaml.Unmarshal(streamToBytes(yamlFile), &f)
		contentJson, err = toJsonStr(f)
	case "register":
		var reg Register
		yaml.Unmarshal(streamToBytes(yamlFile), &reg)
		contentJson, err = toJsonStr(reg)
	case "registry":
		var r Registry
		yaml.Unmarshal(streamToBytes(yamlFile), &r)
		contentJson, err = toJsonStr(r)
	default:
		log.Fatal("register name not recognised")
		return
	}
	if err != nil {
		log.Fatal("failed to marshal to json for " + string(streamToBytes(yamlFile)))
		return
	}

	contentJsonHash := "sha256:" + sha256Hex([]byte(contentJson))
	itemParts := []string{"append-entry", timestamp(), contentJsonHash}
	itemLine := strings.Join(itemParts, "\t")
	entryParts := []string{"add-item", string(contentJson)}
	entryLine := strings.Join(entryParts, "\t")
	fmt.Println(itemLine)
	fmt.Println(entryLine)

}

func main() {
	if len(os.Args) < 3 {
		log.Fatal("Usage: tsv|yaml serializer [fields json file] [data file/directory]")
		return
	}

	log.Println(time.Now())

	fieldsFileName := os.Args[2]
	fieldsFile, fieldsErr := os.Open(fieldsFileName)
	if fieldsErr != nil {
		log.Fatal(fieldsErr)
		return
	}
	defer fieldsFile.Close()

	switch os.Args[1] {

	case "tsv":
		tsvFileName := os.Args[3]
		tsvFile, err := os.Open(tsvFileName)
		if err != nil {
			log.Fatal(err)
			return
		}
		defer tsvFile.Close()
		processCSV(fieldsFile, tsvFile)

	case "yaml":
		yamlDir := os.Args[3]
		registerName := filepath.Base(yamlDir)
		files, err := ioutil.ReadDir(yamlDir)
		if err != nil {
			log.Fatal(err)
			return
		}

		for _, file := range files {
			processYamlFile(file, yamlDir, registerName)
		}

	default:
		log.Fatal("file type was not 'yaml' or 'tsv'")
	}

	log.Println(time.Now())
}
