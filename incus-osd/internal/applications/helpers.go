package applications

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	incusapi "github.com/lxc/incus/v6/shared/api"
	"github.com/lxc/incus/v6/shared/revert"

	"github.com/lxc/incus-os/incus-osd/api"
	"github.com/lxc/incus-os/incus-osd/internal/state"
	"github.com/lxc/incus-os/incus-osd/internal/systemd"
)

// UninstallApplication removes the given application from the state, wipes any local
// data, and removes the sysext image for the application.
func UninstallApplication(ctx context.Context, s *state.State, name string) error {
	// Load the application.
	app, err := Load(ctx, s, name)
	if err != nil {
		return err
	}

	// Can't remove a primary application.
	if app.IsPrimary() {
		return errors.New("cannot remove a primary application")
	}

	// Stop the application.
	err = app.Stop(ctx)
	if err != nil {
		return err
	}

	// Wipe local data.
	err = app.WipeLocalData(ctx)
	if err != nil {
		return err
	}

	// Remove application from the state.
	switch name {
	case "debug":
		s.Applications.Debug = api.Application{}
	case "gpu-support":
		s.Applications.GPUSupport = api.Application{}
	case "incus":
		s.Applications.Incus = api.ApplicationIncus{}
	case "incus-ceph":
		s.Applications.IncusCeph = api.Application{}
	case "incus-linstor":
		s.Applications.IncusLinstor = api.Application{}
	case "migration-manager":
		s.Applications.MigrationManager = api.Application{}
	case "operations-center":
		s.Applications.OperationsCenter = api.Application{}
	default:
		return errors.New("unknown application '" + name + "'")
	}

	// Remove the sysext image.
	err = RemoveExtension(ctx, app)
	if err != nil {
		return err
	}

	// Save the state to disk.
	return s.Save()
}

// StartInitialize starts the specified application, and if needed performs initialization actions.
func StartInitialize(ctx context.Context, s *state.State, appName string) error {
	// Get the application.
	app, err := Load(ctx, s, appName)
	if err != nil {
		return err
	}

	// Start the application.
	slog.InfoContext(ctx, "Starting application", "name", appName, "version", app.Version())

	err = app.Start(ctx)
	if err != nil {
		return err
	}

	// Run initialization if needed.
	if !app.IsInitialized() {
		slog.InfoContext(ctx, "Initializing application", "name", appName, "version", app.Version())

		err = app.Initialize(ctx)
		if err != nil {
			return err
		}
	}

	// If the application has a TLS certificate, print its fingerprint so the user can verify it when initially connecting.
	cert, err := app.GetServerCertificate()
	if err == nil {
		rawFp := sha256.Sum256(cert.Certificate[0])

		slog.InfoContext(ctx, "Application TLS certificate fingerprint", "name", appName, "fingerprint", hex.EncodeToString(rawFp[:]))
	}

	// Save the state to disk.
	return s.Save()
}

// Common helper to construct an HTTP client using the provided local Unix socket.
func unixHTTPClient(socketPath string) (*http.Client, error) {
	// Setup a Unix socket dialer
	unixDial := func(_ context.Context, _ string, _ string) (net.Conn, error) {
		raddr, err := net.ResolveUnixAddr("unix", socketPath)
		if err != nil {
			return nil, err
		}

		return net.DialUnix("unix", nil, raddr)
	}

	// Define the http transport
	transport := &http.Transport{
		DialContext:           unixDial,
		DisableKeepAlives:     true,
		ExpectContinueTimeout: time.Second * 30,
		ResponseHeaderTimeout: time.Second * 3600,
		TLSHandshakeTimeout:   time.Second * 5,
	}

	// Define the http client
	client := &http.Client{}

	client.Transport = transport

	// Setup redirect policy
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		// Replicate the headers
		req.Header = via[len(via)-1].Header // #nosec G119

		return nil
	}

	return client, nil
}

// Common helper for performing REST API calls.
func doRequest(ctx context.Context, socket string, url string, method string, body []byte) ([]byte, error) {
	client, err := unixHTTPClient(socket)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	r := &incusapi.ResponseRaw{}

	err = json.NewDecoder(resp.Body).Decode(r)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK || r.StatusCode != http.StatusOK {
		return nil, errors.New(r.Error)
	}

	ret, err := json.Marshal(r.Metadata)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// Comment helper to compute the SHA256 fingerprint of a PEM-encoded certificate.
func getCertificateFingerprint(certificate string) (string, error) {
	certBlock, _ := pem.Decode([]byte(certificate))
	if certBlock == nil {
		return "", errors.New("cannot parse certificate PEM")
	}

	cert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return "", fmt.Errorf("invalid certificate: %w", err)
	}

	rawFp := sha256.Sum256(cert.Raw)

	return hex.EncodeToString(rawFp[:]), nil
}

func createTarArchive(archiveRoot string, excludePaths []string, archive io.Writer) error {
	zw := gzip.NewWriter(archive)
	tw := tar.NewWriter(zw)

	// Stat the archive root to get the device it's on, so we can limit our walk to just that file system.
	s, err := os.Stat(archiveRoot)
	if err != nil {
		return err
	}

	rootStat, ok := s.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("unable to stat file " + archiveRoot)
	}

	rootDev := rootStat.Dev

	err = filepath.WalkDir(archiveRoot, func(path string, dirEntry fs.DirEntry, _ error) error {
		archiveFilename := strings.TrimPrefix(path, archiveRoot)

		// Skip the root directory and any relative path starting with a path to be excluded.
		if archiveFilename == "" || slices.ContainsFunc(excludePaths, func(s string) bool {
			return strings.HasPrefix(archiveFilename, s)
		}) {
			return nil
		}

		info, err := dirEntry.Info()
		if err != nil {
			return err
		}

		mode := int64(info.Mode().Perm())
		modTime := info.ModTime()

		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return errors.New("unable to stat file " + archiveFilename)
		}

		uid := int(stat.Uid)
		gid := int(stat.Gid)

		// Don't walk other file systems.
		if info.Mode().IsDir() && stat.Dev != rootDev {
			return fs.SkipDir
		}

		switch {
		case info.Mode().IsDir():
			// Create the header for the directory.
			header := &tar.Header{
				Name:     archiveFilename,
				Typeflag: tar.TypeDir,
				Mode:     mode,
				ModTime:  modTime,
				Uid:      uid,
				Gid:      gid,
			}

			// Write the header.
			err := tw.WriteHeader(header)
			if err != nil {
				return err
			}
		case info.Mode()&os.ModeSymlink == os.ModeSymlink:
			// Get the symlink destination.
			dest, err := os.Readlink(path)
			if err != nil {
				return err
			}

			// Create the header for the symlink.
			header := &tar.Header{
				Name:     archiveFilename,
				Typeflag: tar.TypeSymlink,
				Linkname: dest,
				Mode:     mode,
				ModTime:  modTime,
				Uid:      uid,
				Gid:      gid,
			}

			// Write the header.
			err = tw.WriteHeader(header)
			if err != nil {
				return err
			}
		case info.Mode().IsRegular():
			// Open the file.
			// #nosec G304,G122
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			// Create the header for the file.
			header := &tar.Header{
				Name:     archiveFilename,
				Typeflag: tar.TypeReg,
				Mode:     mode,
				ModTime:  modTime,
				Uid:      uid,
				Gid:      gid,
				Size:     info.Size(),
			}

			// Write the header and file contents.
			err = tw.WriteHeader(header)
			if err != nil {
				return err
			}

			_, err = io.Copy(tw, file)
			if err != nil {
				return err
			}
		default:
			return errors.New("unsupported file: " + archiveFilename)
		}

		return nil
	})
	if err != nil {
		return err
	}

	err = tw.Close()
	if err != nil {
		return err
	}

	return zw.Close()
}

func extractTarArchive(ctx context.Context, archiveRoot string, restartUnits []string, archive io.Reader) error {
	reverter := revert.New()
	defer reverter.Fail()

	// Create the new root directory.
	stat, err := os.Stat(archiveRoot)
	if err != nil {
		return err
	}

	newArchiveRoot := strings.TrimSuffix(archiveRoot, "/") + ".new"

	err = os.Mkdir(newArchiveRoot, stat.Mode())
	if err != nil {
		return err
	}

	// If we encounter an error, clean up intermediate state.
	reverter.Add(func() {
		_ = os.RemoveAll(newArchiveRoot)
	})

	// Iterate through each file in the tar archive.
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return err
		}

		// Don't let someone feed us a path traversal escape attack.
		// #nosec G305
		filename := filepath.Join(newArchiveRoot, header.Name)
		if !strings.HasPrefix(filename, newArchiveRoot) {
			return fmt.Errorf("cannot restore file outside of application root '%s' (bad file '%s')", archiveRoot, filename)
		}

		mode := fs.FileMode(header.Mode) // #nosec G115

		switch header.Typeflag {
		case tar.TypeDir:
			err := os.Mkdir(filename, mode)
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			err := os.Symlink(header.Linkname, filename)
			if err != nil {
				return err
			}
		case tar.TypeReg:
			// Write file to disk.
			// #nosec G304
			file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, mode)
			if err != nil {
				return err
			}
			defer file.Close() //nolint:revive

			// Read from the archive in chunks to avoid excessive memory consumption.
			var size int64

			for {
				n, err := io.CopyN(file, tr, 4*1024*1024)
				size += n

				if err != nil {
					if errors.Is(err, io.EOF) {
						break
					}

					return err
				}
			}
		default:
			return errors.New("unsupported file: " + header.Name)
		}

		// Set proper ownership.
		err = os.Lchown(filename, header.Uid, header.Gid)
		if err != nil {
			return err
		}
	}

	// Stop unit(s).
	err = systemd.StopUnit(ctx, restartUnits...)
	if err != nil {
		return err
	}

	// Remove the existing directory.
	err = os.RemoveAll(archiveRoot)
	if err != nil {
		return err
	}

	// Rename the new directory.
	err = os.Rename(newArchiveRoot, archiveRoot)
	if err != nil {
		return err
	}

	// Start unit(s).
	err = systemd.StartUnit(ctx, restartUnits...)
	if err != nil {
		return err
	}

	reverter.Success()

	return nil
}
