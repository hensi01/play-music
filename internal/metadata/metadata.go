package metadata

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/dhowden/tag"
)

// ErrInvalidAudio marks a file that is not decodable audio: junk bytes renamed
// to an audio extension, or a truncated/corrupt file. Its text is the API error
// for a rejected upload.
var ErrInvalidAudio = errors.New("Arquivo de áudio inválido ou corrompido")

type Tags struct {
	Title       string
	Artist      string
	Album       string
	AlbumArtist string
	Genre       string
	Year        int
	Track       int
	Disc        int
	Duration    float64
	Bitrate     int
	SampleRate  int
	Picture     *tag.Picture
}

func read(path string, size int64, ffmpegPath string) (*Tags, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m, err := tag.ReadFrom(f)
	if err != nil {
		m = nil
	}

	t := &Tags{}
	if m != nil {
		t.Title = m.Title()
		t.Artist = m.Artist()
		t.Album = m.Album()
		t.AlbumArtist = m.AlbumArtist()
		t.Genre = m.Genre()
		t.Year = m.Year()
		if trk, _ := m.Track(); trk > 0 {
			t.Track = trk
		}
		if disc, _ := m.Disc(); disc > 0 {
			t.Disc = disc
		}
		if pic := m.Picture(); pic != nil {
			t.Picture = pic
		}
	}

	if strings.EqualFold(filepath.Ext(path), ".wav") && t.Duration <= 0 {
		if d, sr, br := probeWAV(f); d > 0 || sr > 0 || br > 0 {
			t.Duration, t.SampleRate, t.Bitrate = d, sr, br
		}
	}

	ffprobe := probePath(ffmpegPath)
	if ffprobe != "" {
		if d, sr, br := probe(path, ffprobe); d > 0 || sr > 0 || br > 0 {
			t.Duration, t.SampleRate, t.Bitrate = d, sr, br
		}
	}

	if t.Duration <= 0 && m != nil {
		if v, ok := m.Raw()["LENGTH"]; ok {
			if s, ok := v.(string); ok {
				if d, err := strconv.ParseFloat(s, 64); err == nil {
					t.Duration = d
				}
			}
		}
	}
	if t.Duration <= 0 && t.Bitrate > 0 && size > 0 {
		t.Duration = float64(size) * 8 / float64(t.Bitrate)
	}
	if t.Title == "" {
		t.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	t.Title = clean(t.Title)
	t.Artist = clean(t.Artist)
	t.Album = clean(t.Album)
	t.AlbumArtist = clean(t.AlbumArtist)
	t.Genre = clean(t.Genre)

	return t, nil
}

// ReadAudio reads tags from a file that must be decodable audio. A file whose
// tags are unreadable or that reports no duration is not audio at all (junk
// bytes renamed to an audio extension, a truncated download), and comes back
// wrapped in ErrInvalidAudio.
func ReadAudio(path string, size int64, ffmpegPath string) (*Tags, error) {
	t, err := read(path, size, ffmpegPath)
	if err != nil {
		return nil, fmt.Errorf("%w: metadados ilegíveis (%v)", ErrInvalidAudio, err)
	}
	if t.Duration <= 0 {
		return nil, fmt.Errorf("%w: duração não detectada (arquivo sem áudio decodificável)", ErrInvalidAudio)
	}
	return t, nil
}

// clean strips NUL bytes, which Postgres TEXT cannot store.
func clean(s string) string {
	if !strings.ContainsRune(s, 0) {
		return s
	}
	return strings.ReplaceAll(s, "\x00", "")
}

// FFmpegPath resolves the ffmpeg binary: the configured ND_FFMPEGPATH when it
// is a real file, else whatever PATH offers.
func FFmpegPath(configured string) string {
	if configured != "" {
		if fi, err := os.Stat(configured); err == nil && !fi.IsDir() {
			return configured
		}
	}
	if p, err := exec.LookPath("ffmpeg"); err == nil {
		return p
	}
	return ""
}

func probeWAV(f *os.File) (float64, int, int) {
	buf := make([]byte, 4096)
	n, err := f.ReadAt(buf, 0)
	if err != nil && n < 12 {
		return 0, 0, 0
	}
	buf = buf[:n]
	if string(buf[0:4]) != "RIFF" || string(buf[8:12]) != "WAVE" {
		return 0, 0, 0
	}
	var duration float64
	var sampleRate, bitrate int
	var byteRate uint32
	off := 12
	for off+8 <= n {
		size := int(binary.LittleEndian.Uint32(buf[off+4 : off+8]))
		switch string(buf[off : off+4]) {
		case "fmt ":
			if off+26 <= n {
				sampleRate = int(binary.LittleEndian.Uint32(buf[off+12 : off+16]))
				byteRate = binary.LittleEndian.Uint32(buf[off+16 : off+20])
				bitrate = int(byteRate * 8)
			}
		case "data":
			if byteRate > 0 && size > 0 {
				duration = float64(size) / float64(byteRate)
			}
			return duration, sampleRate, bitrate
		}
		off += 8 + size + (size & 1)
	}
	return duration, sampleRate, bitrate
}

func probePath(configured string) string {
	if configured != "" {
		dir := filepath.Dir(configured)
		for _, n := range []string{"ffprobe.exe", "ffprobe"} {
			cand := filepath.Join(dir, n)
			if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
				return cand
			}
		}
	}
	if p, err := exec.LookPath("ffprobe"); err == nil {
		return p
	}
	return ""
}

type probeOutput struct {
	Streams []struct {
		CodecType  string `json:"codec_type"`
		SampleRate string `json:"sample_rate"`
		BitRate    string `json:"bit_rate"`
		Duration   string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
		BitRate  string `json:"bit_rate"`
	} `json:"format"`
}

func probe(path, ffprobe string) (float64, int, int) {
	cmd := exec.Command(ffprobe, "-v", "quiet", "-print_format", "json",
		"-show_format", "-show_streams", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, 0, 0
	}
	var p probeOutput
	if err := json.Unmarshal(out, &p); err != nil {
		return 0, 0, 0
	}
	var duration float64
	var sampleRate, bitrate int
	for _, s := range p.Streams {
		if s.CodecType != "audio" {
			continue
		}
		if d := parseFloat(s.Duration); d > 0 {
			duration = d
		}
		if s.SampleRate != "" {
			if sr, err := strconv.Atoi(s.SampleRate); err == nil && sr > 0 {
				sampleRate = sr
			}
		}
		if br := parseBitrate(s.BitRate); br > 0 {
			bitrate = br
		}
		break
	}
	if duration <= 0 && p.Format.Duration != "" {
		duration = parseFloat(p.Format.Duration)
	}
	if bitrate <= 0 {
		bitrate = parseBitrate(p.Format.BitRate)
	}
	return duration, sampleRate, bitrate
}

func parseFloat(s string) float64 {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func parseBitrate(bitRate string) int {
	if bitRate != "" {
		if br, err := strconv.Atoi(bitRate); err == nil && br > 0 {
			return br
		}
	}
	return 0
}
