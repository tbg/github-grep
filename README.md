# ghg is (experimental) full text offline Github issue search

1. `go get -tags fts5 github.com/tschottdorf/github-grep/pkg/cmd/ghg` 
2. `export GHI_TOKEN='<your api token here>'`
3. `cd $(go env GOPATH)/src/github.com/some/repo)`
4. `ghg`
5. search away! The query uses [sqlite3 `MATCH` syntax](https://www.sqlite.org/fts5.html) (Section 3.2+) but hopefully just typing a few words helps
6. every now and then: `ghg sync`.
