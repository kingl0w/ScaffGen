package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// fileNode represents file or directory in the project structure.
type FileNode struct {
	ID       int
	Name     string
	IsDir    bool
	Children []*FileNode
	Parent   *FileNode
	Depth    int
}

var nodeIDCounter int

func main() {
	outputDirFlag := flag.String("o", "", "Output directory for the generated structure")
	debugFlag := flag.Bool("debug", false, "Enable debug logging")
	flag.Parse()

	err := godotenv.Load()
	if err != nil {
		if *debugFlag {
			fmt.Println("Debug: Error loading .env file:", err)
		}
	}

	args := flag.Args()
	userPrompt := ""
	if len(args) > 0 {
		userPrompt = args[0]
	} else {
		fmt.Println("No initial prompt provided. Please describe your project.")
		userPrompt = readUserInput("Prompt: ")
		if userPrompt == "" {
			fmt.Println("No prompt entered. Exiting.")
			return
		}
	}

	apiKey := os.Getenv("GROQ_API_KEY")
	model := os.Getenv("MODEL")
	groqAPI := os.Getenv("GROQ_API_URL")

	if apiKey == "" || model == "" {
		fmt.Println("Error: GROQ_API_KEY and MODEL environment variables must be set.")
		return
	}
	if groqAPI == "" {
		groqAPI = "https://api.groq.com/openai/v1/chat/completions"
	}

	var projectRootNode *FileNode
	var shouldRepromptLLM bool

	for { //main application loop: llm interaction and user modification
		shouldRepromptLLM = false //reset for each iteration

		rawProjectLayout := getProjectLayout(userPrompt, apiKey, model, groqAPI, *debugFlag)
		if rawProjectLayout == "" {
			fmt.Println("No layout returned from LLM.")
			if !askYesNo("Would you like to try a different prompt? (y/N): ") {
				abort("Aborted by user.")
				return
			}
			userPrompt = readUserInput("New prompt: ")
			if userPrompt == "" {
				abort("No prompt entered.")
				return
			}
			continue //retry with new prompt
		}

		if *debugFlag {
			fmt.Println("\n\033[1;35m--- Raw LLM Output ---\033[0m\n" + rawProjectLayout + "\n\033[1;35m--- End Raw LLM Output ---\033[0m")
		}

		cleanedLayout := cleanProjectStructure(rawProjectLayout)
		//if cleaning significantly reduces content, switch raw output
		if strings.Count(strings.TrimSpace(cleanedLayout), "\n") < 2 && rawProjectLayout != "" {
			if *debugFlag {
				fmt.Println("\033[1;33mCleaned structure was very short, using raw LLM output as fallback.\033[0m")
			}
			cleanedLayout = rawProjectLayout
		}

		if *debugFlag {
			fmt.Println("\n\033[1;36mParsing proposed structure...\033[0m")
		}
		nodeIDCounter = 0
		parsedRoot, parseErr := parseLayoutToNodeTree(cleanedLayout, *debugFlag)
		if parseErr != nil {
			fmt.Printf("\033[1;31mError parsing project layout: %v\033[0m\n", parseErr)
			fmt.Println("Problematic layout snippet:\n", firstNLines(cleanedLayout, 5))
			if !askYesNo("Would you like to try a different prompt? (y/N): ") {
				abort("Aborted due to parsing error.")
				return
			}
			userPrompt = readUserInput("New prompt: ")
			if userPrompt == "" {
				abort("No prompt entered.")
				return
			}
			continue //retry
		}
		projectRootNode = parsedRoot

	interactiveModificationLoop:
		for { //inner loop for user modifications
			fmt.Println("\n\033[1;36mCurrent Project Structure:\033[0m")
			if projectRootNode == nil {
				fmt.Println("\033[1;33m(Structure is empty)\033[0m")
			} else {
				displayNodeTree(projectRootNode, "", true)
			}

			var promptActionText string
			if projectRootNode == nil {
				promptActionText = "\n\033[1;33mActions: [r]e-prompt LLM, [a]bort: \033[0m"
			} else {
				promptActionText = "\n\033[1;33mActions: [c]reate, [d <id>]elete, [r]e-prompt, [a]bort: \033[0m"
			}
			input := strings.TrimSpace(strings.ToLower(readUserInput(promptActionText)))
			parts := strings.Fields(input)

			if len(parts) == 0 {
				if projectRootNode != nil { //if structure exists, no input means proceed to create
					break interactiveModificationLoop
				}
				continue //if empty, reprompt for action
			}
			action := parts[0]

			switch action {
			case "c", "create":
				if projectRootNode == nil {
					fmt.Println("\033[1;31mCannot create: project structure is empty. Try re-prompting.\033[0m")
					continue
				}
				break interactiveModificationLoop
			case "d", "delete":
				if projectRootNode == nil {
					fmt.Println("\033[1;31mStructure is already empty.\033[0m")
					continue
				}
				if len(parts) < 2 {
					fmt.Println("\033[1;31mUsage: d <item_id_to_delete>\033[0m")
					continue
				}
				id, err := strconv.Atoi(parts[1])
				if err != nil {
					fmt.Println("\033[1;31mInvalid ID. Please enter a number.\033[0m")
					continue
				}
				var foundAndDeleted bool
				projectRootNode, foundAndDeleted = deleteNodeByID(projectRootNode, id)
				if foundAndDeleted {
					fmt.Printf("\033[1;32mItem ID %d (and its children) deleted.\033[0m\n", id)
				} else {
					fmt.Printf("\033[1;31mItem ID %d not found.\033[0m\n", id)
				}
			case "r", "re-prompt":
				userPrompt = readUserInput("Enter new prompt for LLM: ")
				if userPrompt == "" {
					fmt.Println("No prompt entered. Keeping current structure.")
					continue
				}
				shouldRepromptLLM = true
				break interactiveModificationLoop
			case "a", "abort":
				abort("Aborted by user.")
				return
			default:
				fmt.Println("\033[1;31mInvalid action.\033[0m")
			}
		} //end of interactiveModificationLoop

		if shouldRepromptLLM {
			continue //continue main application loop to reprompt llm
		}
		break //break main application loop to proceed to creation
	}

	if projectRootNode == nil {
		fmt.Println("\033[1;33mNo project structure to create.\033[0m")
		return
	}

	outputPath := *outputDirFlag
	if outputPath != "" {
		if err := os.MkdirAll(outputPath, 0755); err != nil {
			fmt.Printf("\033[1;31mError creating base output directory %s: %v\033[0m\n", outputPath, err)
			return
		}
		fmt.Printf("\n\033[1;36mOutput directory: %s\033[0m\n", outputPath)
	}

	fmt.Println("\n\033[1;32mCreating final project structure...\033[0m")
	createStructureFromNodeTree(projectRootNode, outputPath, *debugFlag)
	fmt.Println("\033[1;32mProject structure created successfully!\033[0m")
}

func readUserInput(promptText string) string {
	fmt.Print(promptText)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func askYesNo(promptText string) bool {
	return strings.ToLower(readUserInput(promptText)) == "y"
}

func abort(message string) {
	fmt.Printf("\033[1;31m%s\033[0m\n", message)
}

func firstNLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		return strings.Join(lines[:n], "\n") + "\n..."
	}
	return s
}

func getProjectLayout(prompt, apiKey, model, groqAPI string, debug bool) string {
	templatePrompt := fmt.Sprintf(`You are a helpful coding assistant. Based on the following prompt, generate a well structured file and folder layout in a proper tree format with connecting lines.

Please follow these strict formatting rules:
1.  The root of the project should be explicitly named if the user's prompt implies a project name (e.g., "project-name/").
2.  Use proper tree characters: '├──' for items that have siblings below them, '└──' for the last item in a directory.
3.  Use vertical bars '│' for directory indentation.
4.  Use 4 spaces for each level of indentation.
5.  ALWAYS use a trailing slash "/" for directory names (e.g., "folder1/", "subfolder/").
6.  Do NOT use a trailing slash for file names (e.g., "file1.js", "README.md").
7.  Ensure consistent spacing and format like this example:
my-project/
├── src/
│   ├── main.go
│   └── utils/
│       └── helpers.go
├── tests/
│   └── main_test.go
├── .gitignore
└── README.md

IMPORTANT: ONLY return the tree structure. Do not include any explanations, introductions, or notes. Do not use backticks or any other markdown formatting around the tree.

Prompt: %s`, prompt)

	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": templatePrompt},
		},
		"temperature": 0.2,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Println("Error marshalling JSON for API request:", err)
		return ""
	}

	req, err := http.NewRequest("POST", groqAPI, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error creating HTTP request:", err)
		return ""
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	if debug {
		fmt.Println("Debug: Sending request to LLM...")
	}
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("API Request error:", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading API response body:", err)
		return ""
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("API request failed with status %d. Response:\n%s\n", resp.StatusCode, string(body))
		return ""
	}

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("Error unmarshalling API response JSON:", err, "\nResponse body:", string(body))
		return ""
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		if debug {
			fmt.Println("Debug: 'choices' field not found or empty in API response.", "\nResponse body:", string(body))
		}
		return ""
	}
	firstChoice, ok := choices[0].(map[string]interface{})
	if !ok {
		if debug {
			fmt.Println("Debug: First choice is not a map.", "\nResponse body:", string(body))
		}
		return ""
	}
	message, ok := firstChoice["message"].(map[string]interface{})
	if !ok {
		if debug {
			fmt.Println("Debug: 'message' field not found in choice.", "\nResponse body:", string(body))
		}
		return ""
	}
	content, ok := message["content"].(string)
	if !ok {
		if debug {
			fmt.Println("Debug: 'content' field not found in message or not a string.", "\nResponse body:", string(body))
		}
		return ""
	}

	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") && strings.HasSuffix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		if firstNewline := strings.Index(content, "\n"); firstNewline != -1 {
			firstLine := strings.TrimSpace(content[:firstNewline])
			if len(firstLine) > 0 && len(firstLine) < 15 && !strings.ContainsAny(firstLine, "├──└─│/") && !strings.Contains(firstLine, ".") {
				content = content[firstNewline+1:]
			}
		}
	}
	return strings.TrimSpace(content)
}

func cleanProjectStructure(content string) string {
	lines := strings.Split(content, "\n")
	var cleanedLines []string
	inStructure := false //helps to skip leading/trailing non-structure text

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		lowerTrimmedLine := strings.ToLower(trimmedLine)

		//skip common boilerplate
		if strings.HasPrefix(lowerTrimmedLine, "here is") ||
			strings.HasPrefix(lowerTrimmedLine, "here's") ||
			strings.HasPrefix(lowerTrimmedLine, "sure, here") ||
			strings.HasPrefix(lowerTrimmedLine, "certainly, here") ||
			strings.HasPrefix(lowerTrimmedLine, "the following is") ||
			strings.HasPrefix(lowerTrimmedLine, "note:") ||
			strings.HasPrefix(lowerTrimmedLine, "note ") ||
			strings.HasPrefix(lowerTrimmedLine, "```") {
			continue
		}
		if strings.Contains(lowerTrimmedLine, "suggested structure") ||
			strings.Contains(lowerTrimmedLine, "you can adjust this") ||
			strings.Contains(lowerTrimmedLine, "this is just an example") {
			continue
		}

		originalLine := strings.TrimSuffix(line, "\r")

		isLikelyRootItem := !strings.ContainsAny(trimmedLine, " ") && (strings.HasSuffix(trimmedLine, "/") || strings.Contains(trimmedLine, "."))

		if strings.ContainsAny(originalLine, "├──└─│") ||
			(!inStructure && trimmedLine != "" && isLikelyRootItem) ||
			(inStructure && trimmedLine != "") {
			cleanedLines = append(cleanedLines, originalLine)
			inStructure = true
		}
	}
	return strings.Join(cleanedLines, "\n")
}

func parseLayoutToNodeTree(layout string, debug bool) (*FileNode, error) {
	lines := strings.Split(layout, "\n")
	if len(lines) == 0 || strings.TrimSpace(layout) == "" {
		return nil, fmt.Errorf("layout is empty")
	}

	var root *FileNode
	nodeStack := []*FileNode{}

	if debug {
		fmt.Println("\n\033[1;35m--- Parsing Layout to Node Tree ---\033[0m")
	}

	for i, line := range lines {
		originalLine := strings.TrimSuffix(line, "\r")
		trimmedLine := strings.TrimSpace(originalLine)

		if trimmedLine == "" {
			if debug {
				fmt.Printf("Debug L%d: SKIPPING empty line\n", i+1)
			}
			continue
		}

		var itemNameWithSuffix string
		var currentDepth int
		indentPart := ""

		prefixFound := false
		treePrefixes := []string{"├── ", "└── "}
		for _, p := range treePrefixes {
			if idx := strings.Index(originalLine, p); idx != -1 {
				itemNameWithSuffix = strings.TrimSpace(originalLine[idx+len(p):])
				indentPart = originalLine[:idx]
				prefixFound = true
				break
			}
		}

		if prefixFound {
			levelChars := 0
			for _, r := range indentPart {
				if r == '│' || r == ' ' {
					levelChars++
				}
			}
			currentDepth = levelChars / 4
			if itemNameWithSuffix != "" {
				currentDepth++
			}
		} else {
			if root == nil {
				itemNameWithSuffix = trimmedLine
				currentDepth = 0
			} else {
				if debug {
					fmt.Printf("\033[33mDebug L%d: SKIPPING line without tree prefix (root already set): \"%s\"\033[0m\n", i+1, originalLine)
				}
				continue
			}
		}

		if itemNameWithSuffix == "" {
			if debug {
				fmt.Printf("\033[33mDebug L%d: SKIPPING line, could not extract item name from: \"%s\"\033[0m\n", i+1, originalLine)
			}
			continue
		}

		nodeIDCounter++
		newNode := &FileNode{
			ID:    nodeIDCounter,
			Name:  strings.TrimSuffix(itemNameWithSuffix, "/"),
			IsDir: strings.HasSuffix(itemNameWithSuffix, "/"),
			Depth: currentDepth,
		}

		if debug {
			fmt.Printf("Debug L%d: Processed: Name='%s', Depth=%d, IsDir=%v, ID=%d (Raw: '%s')\n",
				i+1, newNode.Name, newNode.Depth, newNode.IsDir, newNode.ID, originalLine)
		}

		if root == nil {
			if newNode.Depth != 0 {
				if debug {
					fmt.Printf("\033[33mDebug L%d: Warning: First item '%s' has depth %d, adjusting to 0.\033[0m\n", i+1, newNode.Name, newNode.Depth)
				}
				newNode.Depth = 0
			}
			root = newNode
			nodeStack = append(nodeStack, root)
		} else {
			for len(nodeStack) > 0 && nodeStack[len(nodeStack)-1].Depth >= newNode.Depth {
				nodeStack = nodeStack[:len(nodeStack)-1]
			}

			if len(nodeStack) == 0 {
				errDetail := fmt.Sprintf("line %d: '%s' (depth %d)", i+1, newNode.Name, newNode.Depth)
				if debug {
					fmt.Printf("\033[31mDebug L%d: Error: Orphaned node or multiple roots detected with '%s'. Current root: '%s'. Node stack empty.\033[0m\n", i+1, newNode.Name, root.Name)
				}
				return nil, fmt.Errorf("invalid tree structure: could not find parent for %s. Structure might have multiple roots or inconsistent indentation", errDetail)
			}

			parentNode := nodeStack[len(nodeStack)-1]
			parentNode.Children = append(parentNode.Children, newNode)
			newNode.Parent = parentNode

			if newNode.IsDir {
				nodeStack = append(nodeStack, newNode)
			}
		}
	}

	if debug {
		fmt.Println("\033[1;35m--- Finished Parsing Layout to Node Tree ---\033[0m")
	}
	if root == nil {
		return nil, fmt.Errorf("failed to parse any valid root node from the layout. The layout might be malformed or empty after cleaning")
	}
	return root, nil
}

func displayNodeTree(node *FileNode, prefix string, isLastChild bool) {
	if node == nil {
		return
	}
	idStr := fmt.Sprintf("[%d]", node.ID)
	fmt.Printf("\033[35m%-5s\033[0m", idStr)

	fmt.Print(prefix)
	if node.Parent != nil {
		if isLastChild {
			fmt.Print("└── ")
			prefix += "    "
		} else {
			fmt.Print("├── ")
			prefix += "│   "
		}
	} else {
		//for root node itself, I want to make sure its name aligns with children,
		//might need to adjust the prefix or print spaces if node.Parent == nil.
		//the root name SHOULD start at column 0.
		//current prefix passed for root is "", so this is probably fine for now, may change later
	}

	if node.IsDir {
		fmt.Printf("\033[1;34m%s/\033[0m\n", node.Name)
	} else {
		fmt.Printf("%s\n", node.Name)
	}

	for i, child := range node.Children {
		displayNodeTree(child, prefix, i == len(node.Children)-1)
	}
}

func deleteNodeByID(root *FileNode, id int) (*FileNode, bool) {
	if root == nil {
		return nil, false
	}
	if root.ID == id {
		return nil, true
	}
	deleted := deleteNodeRecursive(root, id)
	return root, deleted
}

func deleteNodeRecursive(currentParent *FileNode, id int) bool {
	if currentParent == nil {
		return false
	}
	for i, child := range currentParent.Children {
		if child.ID == id {
			currentParent.Children = append(currentParent.Children[:i], currentParent.Children[i+1:]...)
			return true
		}
		if deleteNodeRecursive(child, id) {
			return true
		}
	}
	return false
}

func createStructureFromNodeTree(node *FileNode, currentBasePath string, debug bool) {
	if node == nil {
		return
	}

	itemPath := filepath.Join(currentBasePath, node.Name)

	if node.IsDir {
		err := os.MkdirAll(itemPath, 0755)
		if err != nil {
			fmt.Printf("\033[1;31mError creating directory %s: %v\033[0m\n", itemPath, err)
			return
		}
		fmt.Printf("\033[1;34mCreated directory: %s/\033[0m\n", itemPath)
		for _, child := range node.Children {
			createStructureFromNodeTree(child, itemPath, debug)
		}
	} else {
		parentDir := filepath.Dir(itemPath)
		if parentDir != "." {
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				fmt.Printf("\033[1;31mError creating parent directory %s for file %s: %v\033[0m\n", parentDir, itemPath, err)
				return
			}
		}

		file, err := os.Create(itemPath)
		if err != nil {
			fmt.Printf("\033[1;31mError creating file %s: %v\033[0m\n", itemPath, err)
			return
		}
		file.Close()
		fmt.Printf("\033[1;32mCreated file: %s\033[0m\n", itemPath)
	}
}
