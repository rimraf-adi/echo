package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/echo-vcs/echo/internal/store"
)

// HashFiles computes SHA-256 hashes for all scanned files concurrently using a worker pool
func HashFiles(rootDir string, files map[string]*FileInfo, numWorkers int) error {
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}
	if numWorkers <= 0 {
		numWorkers = 1
	}

	type task struct {
		info *FileInfo
	}

	tasks := make(chan task, len(files))
	for _, fi := range files {
		tasks <- task{info: fi}
	}
	close(tasks)

	var wg sync.WaitGroup
	errCh := make(chan error, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				fullPath := filepath.Join(rootDir, filepath.FromSlash(t.info.Path))

				var hash string
				var err error

				if t.info.Mode&os.ModeSymlink != 0 {
					target, rerr := os.Readlink(fullPath)
					if rerr != nil {
						errCh <- fmt.Errorf("reading symlink %s: %w", t.info.Path, rerr)
						return
					}
					hash = store.HashBytes([]byte(target))
				} else {
					f, ferr := os.Open(fullPath)
					if ferr != nil {
						errCh <- fmt.Errorf("opening file %s: %w", t.info.Path, ferr)
						return
					}
					hash, err = store.HashReader(f)
					_ = f.Close()
					if err != nil {
						errCh <- fmt.Errorf("hashing file %s: %w", t.info.Path, err)
						return
					}
				}

				t.info.Hash = hash
			}
		}()
	}

	wg.Wait()
	close(errCh)

	if err, ok := <-errCh; ok {
		return err
	}

	return nil
}
