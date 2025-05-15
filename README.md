# ScaffGen

**ScaffGen** is a fast, lightweight Go based CLI tool that uses natural language to generate project folder and file structures 

> "Describe your project. Let AI scaffold it."

---

## Features

- Accepts natural language like:  
  `A Spring Boot backend with a Svelte frontend and PostgreSQL`
- Outputs a clean, structured folder layout
- Interactive: delete nodes, retry prompt, modify before building
- Build your file/folder structure instantly in any directory
- 100% local creation — your API key, your machine

---

## Install

```bash
go install github.com/kingl0w/ScaffGen@latest
```

## Usage

In any project folder, run:

```bash
scaffoldgen "I want a Flask API with SQLite and a Svelte frontend"
```

To specify an output folder:

```bash
scaffoldgen -o project-folder "Flask API + React frontend"
```

To enable debug parsing logs:

```bash
scaffoldgen -debug "NestJS backend and Angular admin panel"
```

## Example Output

Current Project Structure:
[1]  lunar-tracking-app/
[2]  ├── backend/
[3]  │   ├── main.go
[4]  │   └── models/
[5]  │       ├── utils/
[6]  │       └── helpers.go
[7]  ├── frontend/
[8]  │   ├── public/
[9]  │   │   └── index.html
[10] │   ├── src/
[11] │   ├── components/
[12] │   │   ├── LunarTracker.js
[13] │   │   └── App.js
[14] │   ├── index.js
[15] │   └── styles/
[16] │       └── App.css
[17] ├── .gitignore
[18] └── README.md

Actions: [c]reate, [d <id>]elete, [r]e-prompt, [a]bort: c

## .env Setup

This tool requires an API key I used Groq and have not tested others.

## How It Works

You describe the project (in plain English).

The app sends the prompt to the Groq API.

The model responds with a structured folder tree.

You review, tweak, and confirm.

ScaffGen creates your project files and folders locally.

## Contributing

Pull requests and suggestions welcome! I built this as a personal tool but hopefully someone else will find it useful

Feel free to fork or submit issues in the GitHub repo.
