# Mermaid Pattern Templates

Mermaid templates for presentation diagrams. Use when precise positioning isn't required and the diagram type fits Mermaid's capabilities.

## When to Use Mermaid vs SVG

| Scenario | Recommendation |
|----------|----------------|
| Flowcharts, decision trees | Mermaid |
| Architecture/system diagrams | Mermaid |
| Sequence diagrams | Mermaid |
| Entity relationships | Mermaid |
| Branded icons/custom shapes | SVG |
| Precise pixel positioning | SVG |
| Complex custom styling | SVG |
| Metrics cards, dashboards | SVG |

## Theme Configuration

### RSAC 2026 Theme

Apply at the start of any Mermaid diagram:

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'primaryColor': '#2464C7',
  'primaryTextColor': '#FFFFFF',
  'primaryBorderColor': '#051464',
  'secondaryColor': '#A6CE38',
  'secondaryTextColor': '#000000',
  'tertiaryColor': '#CCD8EA',
  'tertiaryTextColor': '#000000',
  'lineColor': '#000000',
  'textColor': '#000000',
  'fontFamily': 'Calibri, Arial, sans-serif',
  'fontSize': '18px'
}}}%%
```

### Corporate Theme

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'primaryColor': '#1976D2',
  'primaryTextColor': '#FFFFFF',
  'secondaryColor': '#00897B',
  'tertiaryColor': '#F5F5F5',
  'lineColor': '#212121',
  'textColor': '#212121',
  'fontFamily': 'Arial, Helvetica, sans-serif',
  'fontSize': '18px'
}}}%%
```

### Dark Mode Theme

```mermaid
%%{init: {'theme': 'dark', 'themeVariables': {
  'primaryColor': '#BB86FC',
  'primaryTextColor': '#000000',
  'secondaryColor': '#03DAC6',
  'tertiaryColor': '#1E1E1E',
  'lineColor': '#FFFFFF',
  'textColor': '#FFFFFF',
  'fontFamily': 'Arial, Helvetica, sans-serif',
  'fontSize': '18px'
}}}%%
```

---

## Pattern: Process Flow

### Horizontal Flow (LR)

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'primaryColor': '#2464C7',
  'primaryTextColor': '#FFFFFF',
  'secondaryColor': '#A6CE38',
  'tertiaryColor': '#051464',
  'lineColor': '#000000',
  'fontFamily': 'Calibri, Arial, sans-serif'
}}}%%
flowchart LR
    A[Step 1: Request] --> B[Step 2: Process]
    B --> C[Step 3: Response]

    style A fill:#2464C7,color:#fff
    style B fill:#A6CE38,color:#000
    style C fill:#051464,color:#fff
```

### Vertical Flow (TB)

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'primaryColor': '#2464C7',
  'primaryTextColor': '#FFFFFF',
  'fontFamily': 'Calibri, Arial, sans-serif'
}}}%%
flowchart TB
    A[Start] --> B{Decision}
    B -->|Yes| C[Action A]
    B -->|No| D[Action B]
    C --> E[End]
    D --> E

    style A fill:#2464C7,color:#fff
    style B fill:#D823AD,color:#fff
    style C fill:#A6CE38,color:#000
    style D fill:#23ABAD,color:#000
    style E fill:#051464,color:#fff
```

### With Subgraphs

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'primaryColor': '#2464C7',
  'fontFamily': 'Calibri, Arial, sans-serif'
}}}%%
flowchart LR
    subgraph Input["Input Phase"]
        A[Receive Request]
        B[Validate]
    end

    subgraph Process["Processing Phase"]
        C[Transform]
        D[Enrich]
    end

    subgraph Output["Output Phase"]
        E[Format]
        F[Send Response]
    end

    A --> B --> C --> D --> E --> F
```

---

## Pattern: Architecture Layers

### System Context (C4 Style)

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'primaryColor': '#2464C7',
  'secondaryColor': '#CCD8EA',
  'fontFamily': 'Calibri, Arial, sans-serif'
}}}%%
flowchart TB
    User[("User<br/>External Actor")]

    subgraph System["System Boundary"]
        direction TB
        API[API Gateway]
        Service[Core Service]
        DB[(Database)]
    end

    External[("External API<br/>Third Party")]

    User --> API
    API --> Service
    Service --> DB
    Service --> External

    style User fill:#CCD8EA,color:#000
    style External fill:#CCD8EA,color:#000
    style API fill:#2464C7,color:#fff
    style Service fill:#A6CE38,color:#000
    style DB fill:#051464,color:#fff
```

### Layered Architecture

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'primaryColor': '#2464C7',
  'fontFamily': 'Calibri, Arial, sans-serif'
}}}%%
flowchart TB
    subgraph Presentation["Presentation Layer"]
        UI[Web UI]
        Mobile[Mobile App]
    end

    subgraph Business["Business Layer"]
        API[API Service]
        Auth[Auth Service]
        Core[Core Logic]
    end

    subgraph Data["Data Layer"]
        DB[(Primary DB)]
        Cache[(Cache)]
        Queue[Message Queue]
    end

    UI --> API
    Mobile --> API
    API --> Auth
    API --> Core
    Core --> DB
    Core --> Cache
    Core --> Queue
```

---

## Pattern: Sequence Diagram

### Basic Request/Response

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'actorTextColor': '#000000',
  'actorLineColor': '#2464C7',
  'signalColor': '#000000',
  'fontFamily': 'Calibri, Arial, sans-serif'
}}}%%
sequenceDiagram
    participant Client
    participant API
    participant Service
    participant Database

    Client->>API: Request
    API->>Service: Process
    Service->>Database: Query
    Database-->>Service: Result
    Service-->>API: Response
    API-->>Client: Response
```

### With Authentication

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'fontFamily': 'Calibri, Arial, sans-serif'
}}}%%
sequenceDiagram
    participant User
    participant App
    participant Auth as Auth Service
    participant API

    User->>App: Login Request
    App->>Auth: Authenticate
    Auth-->>App: Token
    App-->>User: Login Success

    User->>App: API Request
    App->>API: Request + Token
    API->>Auth: Validate Token
    Auth-->>API: Valid
    API-->>App: Response
    App-->>User: Display Result
```

### With Notes and Loops

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'fontFamily': 'Calibri, Arial, sans-serif'
}}}%%
sequenceDiagram
    participant A as Service A
    participant B as Service B
    participant C as Service C

    Note over A,C: Initial Setup
    A->>B: Initialize
    B->>C: Configure

    loop Every 30 seconds
        A->>B: Health Check
        B-->>A: Status OK
    end

    Note over A: Error Handling
    A->>B: Request
    B--xA: Error
    A->>A: Retry Logic
    A->>B: Retry Request
    B-->>A: Success
```

---

## Pattern: Entity Relationship

### Basic ERD

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'primaryColor': '#2464C7',
  'fontFamily': 'Calibri, Arial, sans-serif'
}}}%%
erDiagram
    USER ||--o{ ORDER : places
    USER {
        int id PK
        string name
        string email
    }
    ORDER ||--|{ ORDER_ITEM : contains
    ORDER {
        int id PK
        int user_id FK
        date created_at
    }
    ORDER_ITEM }|--|| PRODUCT : references
    ORDER_ITEM {
        int id PK
        int order_id FK
        int product_id FK
        int quantity
    }
    PRODUCT {
        int id PK
        string name
        decimal price
    }
```

---

## Pattern: State Diagram

### Simple State Machine

```mermaid
%%{init: {'theme': 'base', 'themeVariables': {
  'primaryColor': '#2464C7',
  'fontFamily': 'Calibri, Arial, sans-serif'
}}}%%
stateDiagram-v2
    [*] --> Pending
    Pending --> Processing: Start
    Processing --> Completed: Success
    Processing --> Failed: Error
    Failed --> Processing: Retry
    Completed --> [*]
    Failed --> [*]: Max Retries
```

---

## Styling Reference

### Node Styles

```mermaid
flowchart LR
    A[Default Rectangle]
    B([Stadium])
    C[[Subroutine]]
    D[(Database)]
    E((Circle))
    F{Diamond}
    G{{Hexagon}}
```

### Style Application

```mermaid
flowchart LR
    A[Node A]
    B[Node B]

    style A fill:#2464C7,color:#fff,stroke:#051464,stroke-width:2px
    style B fill:#A6CE38,color:#000,stroke:#051464,stroke-width:2px

    linkStyle default stroke:#000,stroke-width:2px
```

### Class-Based Styling

```mermaid
flowchart LR
    A[Primary]:::primary
    B[Secondary]:::secondary
    C[Tertiary]:::tertiary

    classDef primary fill:#2464C7,color:#fff,stroke:#051464
    classDef secondary fill:#A6CE38,color:#000,stroke:#051464
    classDef tertiary fill:#051464,color:#fff,stroke:#000
```

---

## Accessibility Notes for Mermaid

Mermaid diagrams are rendered as SVG, but accessibility depends on the rendering context:

1. **Alt text**: Always provide alt text when embedding diagrams
2. **Font size**: Use `fontSize: '18px'` minimum in theme config
3. **Contrast**: Verify node fill colors meet contrast requirements
4. **Color coding**: Add labels to nodes, don't rely on color alone
5. **Complexity**: Keep diagrams simple; split complex flows

### Recommended Alt Text Format

```markdown
![Process flow showing three steps: Request validation, Processing, Response generation](diagram.svg)
```
