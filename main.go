package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/apoorvam/goterminal" //https://apoorvam.github.io/go-terminal
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type CRDFiles map[string]string

type Config struct {
	ModuleName string   `json:"moduleName"`
	CRDs       CRDFiles `json:"crds"`
}

var knownAPIVersions = []string{
	"v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9", "v10",
	"v1alpha1", "v1alpha2", "v1alpha3", "v1alpha4", "v1alpha5",
	"v2alpha1", "v2alpha2", "v2alpha3", "v2alpha4", "v2alpha5",
	"v3alpha1", "v3alpha2", "v3alpha3", "v3alpha4", "v3alpha5",
	"v1beta1", "v1beta2", "v1beta3", "v1beta4", "v1beta5",
	"v2beta1", "v2beta2", "v2beta3", "v2beta4", "v2beta5",
	"v3beta1", "v3beta2", "v3beta3", "v3beta4", "v3beta5",
}

var debug bool

func main() {
	rawURL := flag.String("url", "", "GitHub Verzeichnis für Raw die Links")
	moduleName := flag.String("name", "", "Modulname")
	configFile := flag.String("config", "", "Pfad zur JSON config")
	debugFlag := flag.Bool("debug", false, "Aktivieren der Debugging-Funktion")
	flag.Parse()

	debug = *debugFlag

	if *rawURL != "" && *moduleName != "" {
		extractRawLinks(*rawURL, *moduleName)
		return
	}

	if *configFile == "" {
		fmt.Println("\033[31mDavid sagt: ahjöö! Da hast du den Konfigurationspfad vergessen.\033[0m")
		os.Exit(1)
	}

	config := loadConfig(*configFile)
	moduleDir := filepath.Join("modules", config.ModuleName)
	os.MkdirAll(filepath.Join(moduleDir, "crds"), os.ModePerm)

	writer := goterminal.New(os.Stdout)

	downloadCRDs(config.CRDs, moduleDir, writer)
	convertCRDs(config.CRDs, moduleDir, writer)
	moveKclFiles(moduleDir, writer)
	removeEmptyDirs(moduleDir)

	writer.Reset()
	fmt.Println("\033[32mDavid sagt: tiptop, Alles erledigt\033[0m")
}

func extractRawLinks(url string, moduleName string) {
	if debug {
		fmt.Printf("\033[33mDebug: David sagt: Ich hol's mir das zeug von: %s\033[0m\n", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("\033[31mDavid sagt: Da hast du einen URL Fehler gemacht: %v\033[0m\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("\033[31mDavid sagt: ahjöö! Anfrage fehlgeschlagen und nu?: %s\033[0m\n", resp.Status)
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Printf("\033[31mDavid sagt: Das HTML ist im eimer 🪣! Fehler beim Lesen: %v\033[0m\n", err)
		return
	}

	script := doc.Find("script[type=\"application/json\"][data-target=\"react-app.embeddedData\"]").First()
	if script.Length() == 0 {
		fmt.Println("\033[31mDavid sagt: ist das ne JSON o.O? nope!\033[0m")
		return
	}

	jsonData := script.Text()
	var payload struct {
		Payload struct {
			Tree struct {
				Items []struct {
					Name string `json:"name"`
					Path string `json:"path"`
				} `json:"items"`
			} `json:"tree"`
		} `json:"payload"`
	}

	err = json.Unmarshal([]byte(jsonData), &payload)
	if err != nil {
		fmt.Printf("\033[31mDavid sagt: JSON Parsing foobar fooo: %v\033[0m\n", err)
		return
	}

	baseRawURL := strings.Replace(url, "https://github.com/", "https://raw.githubusercontent.com/", 1)
	baseRawURL = strings.Replace(baseRawURL, "/tree/", "/", 1)

	if debug {
		fmt.Printf("\033[33mDebug: David sagt: Das ist die URL für Raw data: %s\033[0m\n", baseRawURL)
	}

	crds := make(map[string]string)
	for _, item := range payload.Payload.Tree.Items {
		if strings.HasSuffix(item.Name, ".yaml") {
			rawLink := baseRawURL + "/" + item.Name
			crds[item.Name] = rawLink
			if debug {
				fmt.Printf("\033[33mDebug: David sagt: ick hab da nen Raw Link gefunden: %s\033[0m\n", rawLink)
			}
		}
	}

	config := Config{
		ModuleName: moduleName,
		CRDs:       crds,
	}

	configJSON, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		fmt.Printf("\033[31mDavid sagt: Konnte die JSON nicht erstellen! tja pech... %v\033[0m\n", err)
		return
	}

	configFilePath := filepath.Join("config", moduleName+".json")
	err = os.WriteFile(configFilePath, configJSON, 0644)
	if err != nil {
		fmt.Printf("\033[31mDavid sagt: Konnte die JSON-Datei nicht schreiben: %v\033[0m\n", err)
		return
	}

	fmt.Printf("\033[32mDavid sagt: JSON-Konfiguration gespeichert in %s\033[0m\n", configFilePath)

	config = loadConfig(configFilePath)
	moduleDir := filepath.Join("modules", config.ModuleName)
	os.MkdirAll(filepath.Join(moduleDir, "crds"), os.ModePerm)

	writer := goterminal.New(os.Stdout)

	downloadCRDs(config.CRDs, moduleDir, writer)
	convertCRDs(config.CRDs, moduleDir, writer)
	moveKclFiles(moduleDir, writer)
	removeEmptyDirs(moduleDir)

	writer.Reset()
	fmt.Println("\033[32mDavid sagt: tiptop, Alles erledigt\033[0m")
}

func loadConfig(filePath string) Config {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("\033[31mDavid sagt: Datei nicht gefunden: %s. watt nu! %v\033[0m\n", filePath, err)
		os.Exit(1)
	}
	defer file.Close()

	var config Config
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		fmt.Printf("\033[31mDavid sagt: ick konnte die JSON nicht Dekodierung 🤷🏽‍♂️ %s! %v\033[0m\n", filePath, err)
		os.Exit(1)
	}

	return config
}

func downloadCRDs(crdFiles CRDFiles, baseDir string, writer *goterminal.Writer) {
	for key, url := range crdFiles {
		filePath := filepath.Join(baseDir, "crds", key+".yaml")
		fmt.Fprintf(writer, "\033[34mDavid sagt: Ich lade %s von %s herunter...\033[0m\n", key, url)
		writer.Print()
		err := downloadFile(filePath, url)
		if err != nil {
			fmt.Fprintf(writer, "\033[31mDavid sagt: Ick hab da nen Download Fehler bei %s. Was hast du da für foo gemacht? %v\033[0m\n", url, err)
			writer.Print()
			os.Exit(1)
		}
		writer.Clear()
	}
}

func convertCRDs(crdFiles CRDFiles, baseDir string, writer *goterminal.Writer) {
	for key := range crdFiles {
		inputFile := filepath.Join(baseDir, "crds", key+".yaml")
		apiVersion, err := extractAPIVersionFromName(key, knownAPIVersions)
		if err != nil {
			fmt.Fprintf(writer, "\033[31mDavid sagt: hmmm Version welche version? lol %s! %v\033[0m\n", inputFile, err)
			writer.Print()
			apiVersion = "unbekannte_api_version"
		}

		outputDir := filepath.Join(baseDir, apiVersion)
		os.MkdirAll(outputDir, os.ModePerm)
		outputFile := filepath.Join(outputDir, key+".k")

		fmt.Fprintf(writer, "\033[34mDavid sagt: Konvertiere %s nach %s...\033[0m\n", inputFile, outputFile)
		writer.Print()
		err = runCommand("kcl", "import", "-m", "crd", inputFile, "-o", outputFile)
		if err != nil {
			fmt.Fprintf(writer, "\033[31mDavid sagt: Konvertierungsfehler bei %s! %v\033[0m\n", inputFile, err)
			writer.Print()
			os.Exit(1)
		}
		writer.Clear()
	}
}

func moveKclFiles(baseDir string, writer *goterminal.Writer) {
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(info.Name(), ".k") && !info.IsDir() {
			apiVersion, err := extractAPIVersionFromName(info.Name(), knownAPIVersions)
			if err != nil {
				if debug {
					fmt.Fprintf(writer, "\033[33mDebug: David sagt: Versionsbestimmung gescheitert bei '%s': %v\033[0m\n", info.Name(), err)
					writer.Print()
				}
				apiVersion = "unbekannte_api_version"
			}

			newDir := filepath.Join(baseDir, apiVersion)
			os.MkdirAll(newDir, os.ModePerm)

			newPath := filepath.Join(newDir, info.Name())
			fmt.Fprintf(writer, "\033[34mDavid sagt: Verschiebe %s nach %s...\033[0m\n", path, newPath)
			writer.Print()
			err = os.Rename(path, newPath)
			if err != nil {
				fmt.Fprintf(writer, "\033[31mDavid sagt: konnte die Datei nicht verschieben '%s'! %v\033[0m\n", path, err)
				writer.Print()
			}
			writer.Clear()
		}
		return nil
	})
	if err != nil {
		fmt.Printf("\033[31mDavid sagt: Beim Verschieben ist etwas schief gelaufen: %v\033[0m\n", err)
		os.Exit(1)
	}

	removeRedundantRegexMatch(baseDir, writer)
}

func removeRedundantRegexMatch(baseDir string, writer *goterminal.Writer) {
	for _, apiVersion := range knownAPIVersions {
		dirPath := filepath.Join(baseDir, apiVersion)
		files, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		found := false
		for _, file := range files {
			if strings.HasSuffix(file.Name(), ".k") {
				filePath := filepath.Join(dirPath, file.Name())
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					continue
				}

				if strings.Contains(string(fileContent), "regex_match = regex.match") {
					if found {
						newContent := strings.Replace(string(fileContent), "regex_match = regex.match", "", 1)
						err = os.WriteFile(filePath, []byte(newContent), 0644)
						if err != nil {
							fmt.Fprintf(writer, "\033[31mDavid sagt: Fehler beim Schreiben der Datei '%s'! %v\033[0m\n", filePath, err)
							writer.Print()
						} else {
							if debug {
								fmt.Fprintf(writer, "\033[33mDebug: David sagt: Entferne 'regex_match = regex.match' aus '%s'\033[0m\n", filePath)
								writer.Print()
							}
						}
					} else {
						found = true
					}
				}
			}
		}
	}
}

func removeEmptyDirs(dir string) {
	for {
		var emptyDirs []string

		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				empty, err := isEmptyDir(path)
				if err != nil {
					return err
				}
				if empty {
					emptyDirs = append(emptyDirs, path)
				}
			}
			return nil
		})

		if len(emptyDirs) == 0 {
			break
		}

		for i := len(emptyDirs) - 1; i >= 0; i-- {
			if debug {
				fmt.Printf("\033[33mDebug: David sagt: da haste ein Leeres Verzeichnis '%s' ick vernichte es nu höhöhö...\033[0m\n", emptyDirs[i])
			}
			err := os.Remove(emptyDirs[i])
			if err != nil {
				fmt.Printf("\033[31mDavid sagt: Löschfehler bei Verzeichnis '%s'! %v\033[0m\n", emptyDirs[i], err)
			}
		}
	}
}

func downloadFile(filepath string, url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func runCommand(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	if debug {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("fehler beim Ausführen des Befehls %s: %w", name, err)
	}
	return nil
}

func extractAPIVersionFromName(name string, knownAPIVersions []string) (string, error) {
	if debug {
		fmt.Printf("\033[33mDebug: Prüfe den Namen '%s' auf API Version...\033[0m\n", name)
	}

	re := regexp.MustCompile(`_v([0-9a-zA-Z]+)`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 1 {
		apiVersion := "v" + matches[1]
		if debug {
			fmt.Printf("\033[33mDebug: Gefundene API Version: '%s'\033[0m\n", apiVersion)
		}
		for _, v := range knownAPIVersions {
			if apiVersion == v {
				if debug {
					fmt.Printf("\033[33mDebug: API-Version '%s' ist bekannt\033[0m\n", apiVersion)
				}
				return apiVersion, nil
			}
		}
	}

	if debug {
		fmt.Printf("\033[33mDebug: Keine bekannte Version im Namen '%s' gefunden tja... so ist das halt\033[0m\n", name)
	}
	return "unbekannte_api_version", nil
}

func isEmptyDir(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
