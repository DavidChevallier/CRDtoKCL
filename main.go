package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/apoorvam/goterminal" // https://apoorvam.github.io/go-terminal
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// CRDFiles represents a map of file names to URLs of CRD files.
type CRDFiles map[string]string

// Config represents the configuration for the application.
type Config struct {
	ModuleName string   `json:"moduleName"`
	CRDs       CRDFiles `json:"crds"`
}

// knownAPIVersions is a slice of strings that contains the known API versions.
var knownAPIVersions = []string{
	"v1", "v2", "v3", "v4", "v5", "v6", "v7", "v8", "v9", "v10",
	"v1alpha1", "v1alpha2", "v1alpha3", "v1alpha4", "v1alpha5",
	"v2alpha1", "v2alpha2", "v2alpha3", "v2alpha4", "v2alpha5",
	"v3alpha1", "v3alpha2", "v3alpha3", "v3alpha4", "v3alpha5",
	"v1beta1", "v1beta2", "v1beta3", "v1beta4", "v1beta5",
	"v2beta1", "v2beta2", "v2beta3", "v2beta4", "v2beta5",
	"v3beta1", "v3beta2", "v3beta3", "v3beta4", "v3beta5",
}

// debug is a boolean variable that indicates whether debug mode is enabled or not.
var debug bool

// main is the entry point of the program. It parses command line flags, creates necessary directories,
// loads configuration, downloads and converts CRDs, moves KCL files, removes empty directories,
// and prints a success message.
func main() {
	rawURL := flag.String("url", "", "GitHub directory for raw links")
	moduleName := flag.String("name", "", "Module name")
	configFile := flag.String("config", "", "Path to JSON config")
	debugFlag := flag.Bool("debug", false, "Enable debugging")
	flag.Parse()

	debug = *debugFlag

	os.MkdirAll("modules", os.ModePerm)
	os.MkdirAll("config", os.ModePerm)

	if *rawURL != "" && *moduleName != "" {
		extractRawLinks(*rawURL, *moduleName)
		return
	}

	if *configFile == "" {
		fmt.Println("\033[31mError: Configuration path is missing.\033[0m")
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
	fmt.Println("\033[32mAll tasks completed successfully.\033[0m")
}

// extractRawLinks fetches content from the specified URL, extracts raw links to YAML files,
// and saves them to a JSON configuration file. It performs the following steps:
// 1. Fetches the content from the URL.
// 2. Parses the HTML and extracts the JSON data.
// 3. Constructs the raw data URL.
// 4. Finds the raw links to YAML files and stores them in a map.
// 5. Creates a JSON configuration object with the module name and the raw links.
// 6. Writes the JSON configuration to a file.
// 7. Creates the necessary directories.
// 8. Downloads the CRDs (Custom Resource Definitions) from the raw links.
// 9. Converts the CRDs to Kubernetes YAML format.
// 10. Moves the KCL (Kubernetes Configuration Language) files to the module directory.
// 11. Removes any empty directories.
// 12. Prints a success message.
//
// Parameters:
// - url: The URL to fetch the content from.
// - moduleName: The name of the module.
//
// Note: This function assumes that the URL contains HTML with a script tag containing
// JSON data in the specified format.
//
// Example usage:
// extractRawLinks("https://example.com", "Module")
func extractRawLinks(url string, moduleName string) {
	if debug {
		fmt.Printf("\033[33mDebug: Fetching content from: %s\033[0m\n", url)
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("\033[31mError: Failed to fetch URL: %v\033[0m\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("\033[31mError: Request failed: %s\033[0m\n", resp.Status)
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		fmt.Printf("\033[31mError: Failed to read HTML: %v\033[0m\n", err)
		return
	}

	script := doc.Find("script[type=\"application/json\"][data-target=\"react-app.embeddedData\"]").First()
	if script.Length() == 0 {
		fmt.Println("\033[31mError: JSON data not found.\033[0m")
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
		fmt.Printf("\033[31mError: Failed to parse JSON: %v\033[0m\n", err)
		return
	}

	baseRawURL := strings.Replace(url, "https://github.com/", "https://raw.githubusercontent.com/", 1)
	baseRawURL = strings.Replace(baseRawURL, "/tree/", "/", 1)

	if debug {
		fmt.Printf("\033[33mDebug: Raw data URL: %s\033[0m\n", baseRawURL)
	}

	crds := make(map[string]string)
	for _, item := range payload.Payload.Tree.Items {
		if strings.HasSuffix(item.Name, ".yaml") {
			rawLink := baseRawURL + "/" + item.Name
			crds[item.Name] = rawLink
			if debug {
				fmt.Printf("\033[33mDebug: Found raw link: %s\033[0m\n", rawLink)
			}
		}
	}

	config := Config{
		ModuleName: moduleName,
		CRDs:       crds,
	}

	configJSON, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		fmt.Printf("\033[31mError: Failed to create JSON: %v\033[0m\n", err)
		return
	}

	configFilePath := filepath.Join("config", moduleName+".json")
	err = os.WriteFile(configFilePath, configJSON, 0644)
	if err != nil {
		fmt.Printf("\033[31mError: Failed to write JSON file: %v\033[0m\n", err)
		return
	}

	fmt.Printf("\033[32mJSON configuration saved to %s\033[0m\n", configFilePath)

	config = loadConfig(configFilePath)
	moduleDir := filepath.Join("modules", config.ModuleName)
	os.MkdirAll(filepath.Join(moduleDir, "crds"), os.ModePerm)

	writer := goterminal.New(os.Stdout)

	downloadCRDs(config.CRDs, moduleDir, writer)
	convertCRDs(config.CRDs, moduleDir, writer)
	moveKclFiles(moduleDir, writer)
	removeEmptyDirs(moduleDir)

	writer.Reset()
	fmt.Println("\033[32mAll tasks completed successfully.\033{0m")
}

// loadConfig loads the configuration from the specified file path.
// It returns the loaded configuration or exits the program with an error message if the file is not found or if there is an error decoding the JSON.
//
// Parameters:
// - filePath: The path to the JSON configuration file.
//
// Returns:
// - Config: The loaded configuration.
func loadConfig(filePath string) Config {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("\033[31mError: File not found: %s. %v\033[0m\n", filePath, err)
		os.Exit(1)
	}
	defer file.Close()

	var config Config
	err = json.NewDecoder(file).Decode(&config)
	if err != nil {
		fmt.Printf("\033[31mError: Failed to decode JSON: %s! %v\033[0m\n", filePath, err)
		os.Exit(1)
	}

	return config
}

// downloadCRDs downloads CRD files from the given URLs and saves them in the specified base directory.
// It displays progress messages using the provided writer.
//
// Parameters:
// - crdFiles: A map of CRD file names to URLs.
// - baseDir: The base directory where the CRD files will be saved.
// - writer: A writer for displaying progress messages.
func downloadCRDs(crdFiles CRDFiles, baseDir string, writer *goterminal.Writer) {
	for key, url := range crdFiles {
		filePath := filepath.Join(baseDir, "crds", key+".yaml")
		fmt.Fprintf(writer, "\033[34mDownloading %s from %s...\033[0m\n", key, url)
		writer.Print()
		err := downloadFile(filePath, url)
		if err != nil {
			fmt.Fprintf(writer, "\033[31mError: Download failed for %s: %v\033[0m\n", url, err)
			writer.Print()
			os.Exit(1)
		}
		writer.Clear()
	}
}

// convertCRDs converts the CRD files to a different format and saves them in the corresponding output directory.
// It takes a map of CRD files, the base directory, and a writer for displaying output messages.
// For each CRD file, it determines the API version, creates the output directory if it doesn't exist,
// and converts the file using the "kcl" command-line tool. If any error occurs during the conversion,
// an error message is displayed and the program exits with a non-zero status code.
//
// Parameters:
// - crdFiles: A map of CRD file names to URLs.
// - baseDir: The base directory where the converted files will be saved.
// - writer: A writer for displaying progress messages.
func convertCRDs(crdFiles CRDFiles, baseDir string, writer *goterminal.Writer) {
	for key := range crdFiles {
		inputFile := filepath.Join(baseDir, "crds", key+".yaml")
		apiVersion, err := extractAPIVersionFromName(key, knownAPIVersions)
		if err != nil {
			fmt.Fprintf(writer, "\033[31mError: Failed to determine version for %s: %v\033[0m\n", inputFile, err)
			writer.Print()
			apiVersion = "unknown_api_version"
		}

		outputDir := filepath.Join(baseDir, apiVersion)
		os.MkdirAll(outputDir, os.ModePerm)
		outputFile := filepath.Join(outputDir, key+".k")

		fmt.Fprintf(writer, "\033[34mConverting %s to %s...\033[0m\n", inputFile, outputFile)
		writer.Print()
		err = runCommand("kcl", "import", "-m", "crd", inputFile, "-o", outputFile)
		if err != nil {
			fmt.Fprintf(writer, "\033[31mError: Conversion failed for %s: %v\033[0m\n", inputFile, err)
			writer.Print()
			os.Exit(1)
		}
		writer.Clear()
	}
}

// moveKclFiles moves files with the ".k" extension to a directory based on their API version.
// It walks through the specified base directory and for each file with the ".k" extension,
// it determines the API version based on the file name and moves the file to a subdirectory
// named after the API version. If the API version cannot be determined, the file is moved
// to a subdirectory named "unknown_api_version".
//
// Parameters:
// - baseDir: The base directory to search for files.
// - writer: A pointer to a goterminal.Writer for displaying output messages.
//
// Example:
// moveKclFiles("/path/to/directory", writer)
func moveKclFiles(baseDir string, writer *goterminal.Writer) {
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(info.Name(), ".k") && !info.IsDir() {
			apiVersion, err := extractAPIVersionFromName(info.Name(), knownAPIVersions)
			if err != nil {
				if debug {
					fmt.Fprintf(writer, "\033[33mDebug: Failed to determine version for '%s': %v\033[0m\n", info.Name(), err)
					writer.Print()
				}
				apiVersion = "unknown_api_version"
			}

			newDir := filepath.Join(baseDir, apiVersion)
			os.MkdirAll(newDir, os.ModePerm)

			newPath := filepath.Join(newDir, info.Name())
			fmt.Fprintf(writer, "\033[34mMoving %s to %s...\033[0m\n", path, newPath)
			writer.Print()
			err = os.Rename(path, newPath)
			if err != nil {
				fmt.Fprintf(writer, "\033[31mError: Failed to move file '%s': %v\033{0m\n", path, err)
				writer.Print()
			}
			writer.Clear()
		}
		return nil
	})
	if err != nil {
		fmt.Printf("\033[31mError: Failed to move files: %v\033[0m\n", err)
		os.Exit(1)
	}

	removeRedundantRegexMatch(baseDir, writer)
}

// removeRedundantRegexMatch removes redundant occurrences of the string "regex_match = regex.match" from files in the specified directory.
// It takes the base directory path and a writer for displaying error and debug messages as input.
// The function iterates over known API versions and searches for files with the ".k" extension in each version's directory.
// If a file contains the string "regex_match = regex.match" and it is not the first occurrence in the directory, the function removes it from the file.
// If an error occurs while reading or writing a file, an error message is displayed using the provided writer.
// If the debug flag is enabled, a debug message is displayed when the string is successfully removed from a file.
//
// Parameters:
// - baseDir: The base directory to search for files.
// - writer: A writer for displaying error and debug messages.
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
							fmt.Fprintf(writer, "\033[31mError: Failed to write file '%s': %v\033[0m\n", filePath, err)
							writer.Print()
						} else {
							if debug {
								fmt.Fprintf(writer, "\033[33mDebug: Removed 'regex_match = regex.match' from '%s'\033[0m\n", filePath)
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

// removeEmptyDirs removes all empty directories within the specified directory.
// It recursively walks through the directory and checks if each directory is empty.
// If an empty directory is found, it is removed. The function stops when there are no more empty directories left.
//
// Parameters:
// - dir: The directory path to start the search from.
//
// Example usage:
// removeEmptyDirs("/path/to/directory")
//
// Note: This function does not handle errors related to directory traversal or removal.
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
				fmt.Printf("\033[33mDebug: Removing empty directory '%s'...\033[0m\n", emptyDirs[i])
			}
			err := os.Remove(emptyDirs[i])
			if err != nil {
				fmt.Printf("\033[31mError: Failed to remove directory '%s': %v\033[0m\n", emptyDirs[i], err)
			}
		}
	}
}

// downloadFile downloads a file from the specified URL and saves it to the given filepath.
//
// Parameters:
// - filepath: The path where the file will be saved.
// - url: The URL to download the file from.
//
// Returns:
// - error: An error if the download or file creation fails, nil otherwise.
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

// runCommand executes the specified command with the given arguments.
// It redirects the command's standard output and standard error based on the value of the `debug` variable.
// If `debug` is true, the output is printed to the console. Otherwise, it is discarded.
// If the command fails to run, an error is returned.
//
// Parameters:
// - name: The name of the command to run.
// - arg: The arguments for the command.
//
// Returns:
// - error: An error if the command execution fails, nil otherwise.
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
		return fmt.Errorf("error running command %s: %w", name, err)
	}
	return nil
}

// extractAPIVersionFromName extracts the API version from the given name string.
// It searches for a pattern in the name string and returns the extracted API version.
// If the API version is found and is known (present in the knownAPIVersions slice),
// it returns the API version string. Otherwise, it returns "unknown_api_version".
//
// Parameters:
// - name: The name string to extract the API version from.
// - knownAPIVersions: A slice of known API versions.
//
// Returns:
// - string: The extracted API version or "unknown_api_version" if not found.
// - error: An error if the extraction fails.
//
// Example usage:
// version, err := extractAPIVersionFromName("example_v1alpha1", knownAPIVersions)
func extractAPIVersionFromName(name string, knownAPIVersions []string) (string, error) {
	if debug {
		fmt.Printf("\033[33mDebug: Checking name '%s' for API version...\033[0m\n", name)
	}

	re := regexp.MustCompile(`_v([0-9a-zA-Z]+)`)
	matches := re.FindStringSubmatch(name)
	if len(matches) > 1 {
		apiVersion := "v" + matches[1]
		if debug {
			fmt.Printf("\033[33mDebug: Found API version: '%s'\033[0m\n", apiVersion)
		}
		for _, v := range knownAPIVersions {
			if apiVersion == v {
				if debug {
					fmt.Printf("\033[33mDebug: API version '%s' is known\033[0m\n", apiVersion)
				}
				return apiVersion, nil
			}
		}
	}

	if debug {
		fmt.Printf("\033[33mDebug: No known version found in name '%s'\033[0m\n", name)
	}
	return "unknown_api_version", nil
}

// isEmptyDir checks if a directory is empty.
// It takes a string parameter `name` representing the directory path.
// It returns a boolean value indicating whether the directory is empty or not,
// and an error if any occurred during the process.
//
// Parameters:
// - name: The path of the directory to check.
//
// Returns:
// - bool: True if the directory is empty, false otherwise.
// - error: An error if the directory cannot be read.
//
// Example usage:
// empty, err := isEmptyDir("/path/to/directory")
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
