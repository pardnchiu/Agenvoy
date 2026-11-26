package record

import (
	"bytes"
	"os"

	"github.com/pardnchiu/agenvoy/configs"
	"github.com/pardnchiu/agenvoy/internal/filesystem"
	utilsmsg "github.com/pardnchiu/agenvoy/internal/utils/msg"
)

func TrimLog() error {
	file, err := os.OpenFile(filesystem.DaemonLogPath, os.O_RDWR, 0600)
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "os.OpenFile", filesystem.DaemonLogPath, err)
	}
	defer file.Close()

	if err := file.Chmod(0600); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "file.Chmod", filesystem.DaemonLogPath, err)
	}

	stat, err := file.Stat()
	if err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "file.Stat", filesystem.DaemonLogPath, err)
	}
	if stat.Size() <= configs.MAX_DOCUMENT_BYTES {
		return nil
	}

	raw := make([]byte, configs.MAX_DOCUMENT_BYTES*3/4)
	if _, err := file.ReadAt(raw, stat.Size()-configs.MAX_DOCUMENT_BYTES*3/4); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "file.ReadAt", filesystem.DaemonLogPath, err)
	}
	if i := bytes.IndexByte(raw, '\n'); i >= 0 {
		raw = raw[i+1:]
	}

	if _, err := file.WriteAt(raw, 0); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "file.WriteAt", filesystem.DaemonLogPath, err)
	}
	if err := file.Truncate(int64(len(raw))); err != nil {
		return utilsmsg.Wrap(utilsmsg.Internal, "file.Truncate", filesystem.DaemonLogPath, err)
	}
	return nil
}
