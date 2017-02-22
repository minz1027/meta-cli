package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"runtime/debug"
	"strconv"

	"github.com/urfave/cli"
)

// VERSION gets set by the build script via the LDFLAGS
var VERSION string

var mkdirAll = os.MkdirAll
var stat = os.Stat
var writeFile = ioutil.WriteFile
var readFile = ioutil.ReadFile
var fprintf = fmt.Fprintf

const metaFile = "meta.json"

// Get meta from file based on key
func getMeta(key string, metaSpace string, output io.Writer) {
	metaFilePath := metaSpace + "/" + metaFile
	metaJson, err := readFile(metaFilePath)
	if err != nil {
		panic(err)
	}
	var metaInterface map[string]interface{}
	err = json.Unmarshal(metaJson, &metaInterface)
	if err != nil {
		panic(err)
	}

	result := metaInterface[key]
	switch result.(type) {
	case map[string]interface{}, []interface{}:
		resultJson, _ := json.Marshal(result)
		fprintf(output, "%v", string(resultJson[:]))
	case nil:
		fprintf(output, "null")
	default:
		fprintf(output, "%v", result)
	}
}

// Store meta to file with key and value
func setMeta(key string, value string, metaSpace string) {
	metaFilePath := metaSpace + "/" + metaFile
	var previousMeta map[string]interface{}

	_, err := stat(metaFilePath)
	// Not exist directory
	if err != nil {
		setupDir(metaSpace)
		// Initialize interface if first setting meta
		previousMeta = make(map[string]interface{})
	} else {
		metaJson, _ := readFile(metaFilePath)
		// Exist meta.json
		if len(metaJson) != 0 {
			err = json.Unmarshal(metaJson, &previousMeta)
			if err != nil {
				panic(err)
			}
		} else {
			// Exist meta.json but it is empty
			previousMeta = make(map[string]interface{})
		}
	}

	key, parsedValue := parseMetaValue(key, value, previousMeta)
	previousMeta[key] = parsedValue

	resultJson, err := json.Marshal(previousMeta)

	err = writeFile(metaFilePath, resultJson, 0666)
	if err != nil {
		panic(err)
	}
}

// Parse arguments of meta-cli to JSON
func parseMetaValue(key string, value string, previousMeta interface{}) (string, interface{}) {
	for p, c := range key {
		if string([]rune{c}) == "[" {
			nextChar := key[p+1]
			if nextChar == []byte("]")[0] {
				// Value is array
				var a [1]interface{}
				key = key[0:p] + key[p+2:] // Remove bracket[] from key
				key, a[0] = parseMetaValue(key, value, previousMeta)
				return key, a
			} else {
				// Value is array with index
				var i int
				for i = p + 1; ; i++ {
					_, err := strconv.Atoi(string(key[i]))
					if err != nil {
						break
					}
				}
				metaIndex, _ := strconv.Atoi(key[p+1 : i]) // e.g. if array[10], get "10"
				key = key[0:p] + key[i+1:]                 // Remove bracket[num] from key

				// Convert previousMeta Interface to Map
				v := reflect.ValueOf(previousMeta)
				var previousMetaMap map[string]interface{}
				previousMetaMap = make(map[string]interface{})
				var previousKey string
				if v.Kind() == reflect.Map {
					for _, k := range v.MapKeys() {
						previousKey, _ = k.Interface().(string)
						previousMetaMap[previousKey] = v.MapIndex(k).Interface()
					}
				}

				var a []interface{}

				// previousMeta[previousKey] is empty or string, create array with null except "value"
				if (previousMetaMap[previousKey] == nil && v.Kind() != reflect.Slice) || reflect.ValueOf(previousMetaMap[previousKey]).Kind() == reflect.String {
					a = make([]interface{}, metaIndex+1)
					key, a[metaIndex] = parseMetaValue(key, value, previousMetaMap[previousKey])
					return key, a
				} else if v.Kind() == reflect.Slice {
					// 既存のJSONに指定されたキーの値があるとき
					// 値が配列
					fmt.Println("Sliceeeeeeeeee", v.Len())
					if metaIndex+1 > v.Len() {
						a = make([]interface{}, metaIndex+1)
						key, a[metaIndex] = parseMetaValue(key, value, nil)
					} else {
						a = make([]interface{}, v.Len())
						key, a[metaIndex] = parseMetaValue(key, value, v.Index(metaIndex).Interface())
					}
					fmt.Println("sliced array", v, v.Index(metaIndex).Interface())
					fmt.Println("a[metaIndex]", a[metaIndex])

					fmt.Println("metaIndex", metaIndex)
					for i := 0; i < v.Len(); i++ {
						if i != metaIndex {
							a[i] = v.Index(i).Interface()
						}
						fmt.Println("i1, v[i]", i, v.Index(i).Interface())
						fmt.Println("i1, a[i]", i, a[i])
					}
					fmt.Println(a)
					return key, a
				} else {
					// 既存のJSONに指定されたキーの値があるとき
					a = make([]interface{}, metaIndex+1)
					prev := reflect.ValueOf(previousMetaMap[previousKey])
					if metaIndex+1 > prev.Len() {
						a = make([]interface{}, metaIndex+1)
						key, a[metaIndex] = parseMetaValue(key, value, nil)
					} else {
						a = make([]interface{}, prev.Len())
						key, a[metaIndex] = parseMetaValue(key, value, prev.Index(metaIndex).Interface())
					}

					fmt.Println("metaIndex", metaIndex)
					// Nothing to do when previous value is object (object(map) to array logic)
					if prev.Kind() != reflect.Map {
						for i := 0; i < prev.Len(); i++ {
							if i != metaIndex {
								a[i] = prev.Index(i).Interface()
							}
							fmt.Println("i, prev[i]", i, prev.Index(i).Interface())
							fmt.Println("i, a[i]", i, a[i])
						}
					}

					fmt.Println("key a[metaIndex]", key, a[metaIndex])
					fmt.Println("previousMeta", key, previousMetaMap[previousKey])
					return key, a
				}
			}
		} else if string([]rune{c}) == "." {
			// Value is object
			childKey := key[p+1:]
			key = key[0:p]
			fmt.Println("childKey", childKey)
			var obj map[string]interface{}
			obj = make(map[string]interface{})
			childKey, tmpValue := parseMetaValue(childKey, value, previousMeta)
			obj[childKey] = tmpValue
			return key, obj
		}
	}
	// Value is int
	i, err := strconv.Atoi(value)
	if err == nil {
		return key, i
	}
	// Value is float
	f, err := strconv.ParseFloat(value, 64)
	if err == nil {
		return key, f
	}
	// Value is bool
	b, err := strconv.ParseBool(value)
	if err == nil {
		return key, b
	}
	// Value is string
	return key, value
}

// setupDir makes directory and json file for meta
func setupDir(metaSpace string) {
	err := mkdirAll(metaSpace, 0777)
	if err != nil {
		panic(err)
	}
	err = writeFile(metaSpace+"/"+metaFile, []byte(""), 0666)
	if err != nil {
		panic(err)
	}
}

var cleanExit = func() {
	os.Exit(0)
}

// finalRecover makes one last attempt to recover from a panic.
// This should only happen if the previous recovery caused a panic.
func finalRecover() {
	if p := recover(); p != nil {
		fmt.Fprintln(os.Stderr, "ERROR: Something terrible has happened. Please file a ticket with this info:")
		fmt.Fprintf(os.Stderr, "ERROR: %v\n%v\n", p, string(debug.Stack()))
	}
	cleanExit()
}

func main() {
	defer finalRecover()

	var metaSpace string

	app := cli.NewApp()
	app.Name = "meta-cli"
	app.Usage = "get or set metadata for Screwdriver build"
	app.UsageText = "meta command arguments [options]"
	app.Copyright = "(c) 2017 Yahoo Inc."

	if VERSION == "" {
		VERSION = "0.0.0"
	}
	app.Version = VERSION

	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "meta-space",
			Usage:       "Location of meta temporarily",
			Value:       "/sd/meta",
			Destination: &metaSpace,
		},
	}

	app.Commands = []cli.Command{
		{
			Name:  "get",
			Usage: "Get a metadata with key",
			Action: func(c *cli.Context) error {
				if len(c.Args()) == 0 {
					return cli.ShowAppHelp(c)
				}
				getMeta(c.Args().First(), metaSpace, os.Stdout)
				return nil
			},
			Flags: app.Flags,
		},
		{
			Name:  "set",
			Usage: "Set a metadata with key and value",
			Action: func(c *cli.Context) error {
				if len(c.Args()) <= 1 {
					return cli.ShowAppHelp(c)
				}
				setMeta(c.Args().Get(0), c.Args().Get(1), metaSpace)
				return nil
			},
			Flags: app.Flags,
		},
	}

	app.Run(os.Args)
}
