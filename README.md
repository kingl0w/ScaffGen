# ScaffGen

**ScaffGen** is a fast, lightweight Go based CLI tool that uses natural language to generate project folder and file structures 

> "Describe your project. Let AI scaffold it."

---

## Features

- Accepts natural language like:  
  `A Flask API with SQLite and a Svelte frontend`
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
ScaffGen "I want a Flask API with SQLite and a Svelte frontend"
```

To specify an output folder:

```bash
ScaffGen -o project-folder "Flask API + React frontend"
```

To enable debug parsing logs:

```bash
ScaffGen -debug "NestJS backend and Angular admin panel"
```

## Example Output


```plaintext
Current Project Structure:
[1]  flask-svelte-api/
[2]  ├── backend/
[3]  │   ├── app.py
[4]  │   ├── __init__.py
[5]  │   ├── models.py
[6]  │   ├── routes.py
[7]  │   ├── config.py
[8]  │   └── database.db
[9]  ├── frontend/
[10] │   ├── public/
[11] │   │   └── index.html
[12] │   ├── src/
[13] │   │   ├── main.js
[14] │   │   ├── components/
[15] │   │   │   └── App.svelte
[16] │   │   └── main.css
[17] │   └── .gitignore
[18] └── README.md

Actions: [c]reate, [d <id>]elete, [r]e-prompt, [a]bort: 
```

## .env Setup

This tool requires an API key I used Groq and have not tested others.

## How It Works

You describe the project (in plain English).

The app sends the prompt to the Groq API.

The model responds with a structured folder tree.

You review, tweak, and confirm.

ScaffGen creates your project files and folders locally.

## Contributing

Pull requests and suggestions welcome. I built this as a personal tool but hopefully someone else will find it useful

Feel free to fork or submit issues in the GitHub repo.
