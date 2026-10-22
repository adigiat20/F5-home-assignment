

## Project structure:

F5-home-assignment
.
├── configs/                  
│   ├── actions.go             # Redaction execution logic (mask, hash, remove, partial)
│   ├── Config.go              # Configuration structures and parser
│   ├── Redactor.go            # Engine implementation & recursive traversal
│   ├── redactor_test.go       # Unit tests for redaction engine & edge cases
│   └── Valuerules.go          # Regex & field-level validation rules (Luhn, JWT, etc.)
├── k8s/                       # Kubernetes manifests
│   └── redactor.yaml          # ConfigMap, Deployment, and ClusterIP Service definitions
├── stream/                    
│   ├── input.ndjson           # Raw test NDJSON stream
│   
├── .dockerignore              # Files excluded from Docker context
├── .gitignore                 # Files excluded from version control
├── Dockerfile                 # Multi-stage Docker build setup
├── go.mod                     # Go module definitions
├── go.sum                     # Go module checksums & dependencies
├── main.go                    # Application entrypoint & CLI streaming 
├── README.md                  # Project documentation
├── rules.yaml                 # Rules configuration
└── Server.go                  # HTTP server mode implementation (/redact, /healthz)

## How to build, run, and test :
So how to run and build?
Prerequisites
Go (1.20+)
Docker
Run via CLI (Local Executable):

- go build -o redactor.exe .

- .\redactor.exe --rules rules.yaml stream\input.ndjson

Now we build and run using Docker:
 - docker build -t redactor-service .    

running input.ndjson (mode http port 8080):
- docker run -p 8080:8080 redactor:latest --serve --addr :8080 --rules rules.yaml

- Invoke-RestMethod -Uri "http://localhost:8080/redact" -Method Post -InFile "stream\input.ndjson"

(running the tests include inside Dockerfile)

## A health endpoint  (Git Bash)
- curl http://localhost:8080/healthz

What i found about the JWT report and what i did?
  - During pattern rule implementation, we updated token decoding to strictly use Base64URL and not Base64 

  ## Docker and Kubernetes deployment:
Multi-stage Docker build with CI-in-a-box:

The Dockerfile employs a multi-stage build design to guarantee code quality:

Builder Stage: Automatically runs go test ./... prior to compilation. If any test fails, the build breaks immediately—preventing faulty code from ever reaching container registries.

Runtime Stage: Copies only the minimal compiled binary into a lightweight image for optimal security and footprint.

  [ Dockerfile ] 
      │
      ▼ 
[ Container Image ]
      │
      ▼ (k8s/redactor.yaml)
[ Pod ] 
      │
      ▼ 
[ Kubernetes Cluster ]

Kubernetes ConfigMap for Configuration (rules.yaml):
What We Chose: Mounting rules.yaml dynamically from a Kubernetes ConfigMap into /etc/redactor/.

Why:

Decoupling Code from Policy: Separates the redaction engine logic from operational business rules. Rule updates do not require rebuilding or redeploying container images.

Native Zero-Downtime Support: Mounting the entire directory allows Kubernetes to update the volume symlink dynamically upon ConfigMap changes, enabling seamless hot-reloading in memory without restarting pods.

What We Chose: A Deployment manifest configured with replicas: 2, ClusterIP Service, non-root security context, explicit resource limits, and health probes (/healthz).

Why:

High Availability & Zero Downtime: Running multiple replicas behind a ClusterIP load balancer guarantees service availability during rolling updates or pod failures.

  ## what you'd do differently with more time :
  I would try to connect the metrics Endpoint(bonus) to App like Grafana so my team could see all the data in dashboard

 ## If this had to run inline on live production traffic with a sub-millisecond budget per request, what would i change?
  To process every request in under 1 millisecond, I would stop turning the JSON into Go objects (map[string]any) .

## Design decisions and trade-offs
The guiding principle: when something is uncertain, hide or drop, never pass through. A redactor's output is what gets sent onward, so losing data is recoverable and leaking it is not.

Recursion: decode to any, walk the tree

Each line is decoded into any and walked recursively (walk in redactor.go): maps, slices and strings are handled, numbers/bools/null are returned untouched.

Why this approach. It is short and easy to reason about, and it handles every shape the spec allows (body can be an object, array, string, number, null or non-JSON text) with one code path.

Scope travels down with the recursion. walk carries a scope (record, headers, body, meta) so a key rule can say "headers only" or "body only". A key named headers nested inside a body stays body-scoped; only the top-level and response.headers objects are header scope.

The function remembers where it is (headers or body), so a rule can say "only headers".
Some bodies are JSON inside a string (like response.body). Those strings are parsed and cleaned as JSON too, so a token key inside is found by name.
Numbers keep their exact text (10.50 stays 10.50).
Trade-offs: the whole line is kept in memory; output keys come out sorted (same data, different order); if a key appears twice, only the last one is kept; only values are checked by value rules, not key names.

2. Streaming: one line at a time
The tool reads a line, cleans it, writes it, and forgets it. Memory depends on the longest line, not the file size. Time grows linearly with the input size.
Go's regex engine can't get stuck on bad input (no catastrophic backtracking), so a hostile line can't freeze it.
The CLI and the HTTP service use the same loop.
Trade-offs: no limit on line length (one giant line is fully loaded); output is buffered, so a slow sender doesn't see results instantly; blank lines are skipped.

3. Rule matching: clear order of priority
Key rules match a key or header name. Value rules match the text of any string.
If a key rule matches, it replaces the whole value and value rules don't run on it. First matching key rule wins.
Value rules all look at the original text. If two overlap, the one that starts first wins (then the one listed first). So one rule never matches the output of another.
A regex finds candidates; a validator can say no. Cards: the regex finds 13-19 digits, then the Luhn check decides, so a random 16-digit number is left alone.
Trade-off: the tool only hides what the rules describe. It doesn't guess.

4. Failure handling: stop or drop, never leak
Bad rules file (missing, broken, empty, bad regex) → the program exits before reading anything or listening. In Kubernetes the pod never becomes Ready, so no traffic reaches it.
A line that isn't valid JSON (for example a cut-off line) is dropped with a warning. Why: a half-line can hide a secret from every regex, so passing it through could leak, and "best-effort" cleaning can't be trusted. The warning shows only the line number, never the content. The cost: output can have fewer lines than input.
Valid JSON with a strange body (form data, plain text, binary-looking text) is not an error; it is cleaned as text.
Sensitive data is never written to stderr or logs.

5. Running it twice (idempotency)

Cleaning the output again gives the same result, except when a key rule uses hash: the second run hashes the hash. A hash looks like any 64-character hex secret, so "skip hashes" would leak real hex keys. Planned fix: write hashes as sha256:<hex> and skip only those.
Rules (rules.yaml)

Provided format, with two regex fixes (marked FIX:): jwt now accepts - and _ (JWTs use base64url), and credit-card accepts spaces/dashes between digits.

Service and deployment
POST /redact: NDJSON in, cleaned NDJSON out, streamed. GET /healthz: 200 means "running and rules loaded".
Dockerfile: small static image, runs as non-root; rules are not inside the image.
deploy/: ConfigMap (rules), Deployment
## Bonuses 

### 1. Prometheus-Style Metrics
The HTTP server exposes operational counters at the `/metrics` endpoint in a Prometheus-compatible format to enable real-time monitoring and observability.

#### Exposed Metrics:
- `redacted_bytes_total`: Total volume of processed data in bytes.
- `redactions_total`: Aggregate count of triggered redaction events across all rules.

#### How to Test locally with Docker:

Concurrency (worker pool) with preserved output order and correctness guarantees
1. Implements a concurrent, thread-safe worker pool (runConcurrent) that processes NDJSON streams in parallel across multi-core systems.
Utilizes a bounded channel pipeline to guarantee strict output line ordering

Prometheus-style metrics
2. **Build the updated image:**
   ```bash
   - docker build -t redactor:latest .
   - docker run -p 8081:8080 redactor:latest --serve --addr :8080 --rules rules.yaml
   open a new terminal:
   -curl http://localhost:8081/metrics