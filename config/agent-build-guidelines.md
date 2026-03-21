Purpose: Provide a consistent, cross-project reference for any agent tasked with building software components.

1. Architecture Principles

Follow SOLID principles: single responsibility, open/closed, Liskov substitution, interface segregation, dependency inversion.

Follow Clean Architecture layering:

Entities / Core logic – pure business rules, no external dependencies.

Use Cases / Application logic – orchestrate workflows.

Interface Adapters – translate data between core logic and external systems (CLI, DB, APIs).

Frameworks & Drivers – external systems like databases, APIs, file systems.

Dependencies must always point inward, never from core logic to infrastructure.

Ensure modularity: each component can be tested and replaced independently.

2. Language Guidelines by Use Case

CLI Tools: Always implement in Go for cross-platform binaries, performance, and minimal runtime dependencies.

Data Processing / ML / Embeddings: Use Python where libraries or dynamic processing are needed.

Support Scripts / Orchestration: Use whichever language is most effective; prefer Python for short scripts.

Agent Decision Rule: “If a component is meant to be a standalone CLI or deployable executable, use Go; otherwise, Python is acceptable.”

3. Storage & Tooling

Lightweight local storage: SQLite

Vector or structured databases: optional, depending on project scale.

Prefer standard libraries unless specialized functionality is required.

Messaging / orchestration should minimize dependencies.

4. Pipeline / Layer Guidelines (if applicable)

Ingestion / data collection

Parsing / preprocessing

Indexing / transformation

Storage / persistence

Query / retrieval

Keep layers loosely coupled with clear interfaces.

5. Cross-Project Notes for Agents

Apply this guideline set to any new project by default.

Language, storage, and architecture rules should be consistent across projects.

CLI components are always Go; Python is secondary for data-heavy or scripting tasks.

Maintain modularity, SOLID design, and clear layer separation in every build.



Indexer/
│
├── cmd/                 # CLI entry points
│   ├── indexer.go       # main CLI tool (Go)
│   └── other_tools.go   # optional additional Go CLI tools
│
├── internal/            # all core code hidden from external packages
│   ├── core/            # entities & domain logic (pure Go)
│   ├── usecases/        # orchestrates core logic (Go or Python)
│   ├── adapters/        # interface adapters (DB, file IO, APIs)
│   └── infrastructure/  # DB connections, storage, frameworks
│
├── scripts/             # Python scripts for preprocessing, embeddings, ML
│   └── preprocess.py
│
├── data/                # default folder for SQLite DBs or persistent storage
│
├── configs/             # configuration files
│   └── indexer.yaml
│
├── tests/               # unit / integration tests
│   ├── core/
│   ├── usecases/
│   └── adapters/
│
└── README.md            # project overview