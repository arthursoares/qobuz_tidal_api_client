"""Unit tests for the downloader's pure helpers and config dataclasses.

The full AlbumDownloader pipeline is exercised end-to-end against a real
Tidal account in a separate credential-gated test (not included here).
"""

import base64

import pytest

from tidal.downloader import (
    AlbumResult,
    DownloadConfig,
    TrackResult,
    _build_albumartist,
    _decrypt_mqa,
    _safe_filename,
    _safe_value,
    _zero_pad,
)
from tidal.types import Album, ArtistSummary


class TestSafeValue:
    def test_replaces_slash_with_dash(self):
        assert _safe_value("AC/DC") == "AC - DC"

    def test_strips_reserved_chars(self):
        assert _safe_value("Hello<World>:!?*") == "HelloWorld!"

    def test_removes_newlines(self):
        assert _safe_value("Line1\nLine2") == "Line1 Line2"

    def test_strips_carriage_returns(self):
        assert _safe_value("a\rb") == "ab"

    def test_truncates_to_200_chars(self):
        long = "x" * 500
        assert len(_safe_value(long)) == 200

    def test_strips_leading_and_trailing_whitespace(self):
        assert _safe_value("  hello  ") == "hello"


class TestSafeFilename:
    def test_strips_trailing_dots(self):
        assert _safe_filename("hello...") == "hello"

    def test_strips_trailing_spaces(self):
        assert _safe_filename("hello   ") == "hello"

    def test_handles_combined_trailing(self):
        assert _safe_filename("hello. .") == "hello"


class TestZeroPad:
    def test_pads_single_digit(self):
        assert _zero_pad(3) == "03"

    def test_does_not_pad_double_digit(self):
        assert _zero_pad(15) == "15"

    def test_custom_width(self):
        assert _zero_pad(5, width=4) == "0005"


class TestBuildAlbumartist:
    def test_returns_artist_name(self):
        album = Album(
            id=1,
            title="X",
            artist=ArtistSummary(id=1, name="Pink Floyd"),
        )
        assert _build_albumartist(album) == "Pink Floyd"

    def test_falls_back_to_unknown_when_empty(self):
        album = Album(
            id=1,
            title="X",
            artist=ArtistSummary(id=0, name=""),
        )
        assert _build_albumartist(album) == "Unknown"


class TestDownloadConfig:
    def test_defaults_match_qobuz_sdk_shape(self):
        cfg = DownloadConfig(output_dir="/music")
        assert cfg.quality == 3
        assert cfg.max_connections == 6
        assert cfg.embed_cover is True
        assert cfg.disc_subdirectories is True
        assert cfg.skip_downloaded is True
        assert cfg.tag_files is True

    def test_can_override_all_fields(self):
        cfg = DownloadConfig(
            output_dir="/music",
            quality=2,
            max_connections=10,
            embed_cover=False,
            tag_files=False,
        )
        assert cfg.quality == 2
        assert cfg.max_connections == 10
        assert cfg.embed_cover is False
        assert cfg.tag_files is False


class TestAlbumResult:
    def test_success_rate_zero_total(self):
        result = AlbumResult(
            album_id=1, title="x", artist="y", total=0, successful=0
        )
        assert result.success_rate == 0.0

    def test_success_rate_full(self):
        result = AlbumResult(
            album_id=1, title="x", artist="y", total=10, successful=10
        )
        assert result.success_rate == 1.0

    def test_success_rate_partial(self):
        result = AlbumResult(
            album_id=1, title="x", artist="y", total=10, successful=8
        )
        assert result.success_rate == 0.8


class TestTrackResult:
    def test_success_track(self):
        tr = TrackResult(
            track_id=1, title="x", success=True, file_path="/tmp/x.flac"
        )
        assert tr.success is True
        assert tr.error is None

    def test_failure_with_error(self):
        tr = TrackResult(
            track_id=1, title="x", success=False, error="region locked"
        )
        assert tr.error == "region locked"


class TestMqaDecryption:
    """Round-trip the AES-CTR decryption against a known plaintext.

    We can't predict the plaintext from a real Tidal payload (the master
    key is fixed but the IV/key are random per track), but we can verify
    the function runs without raising on a synthetic payload built using
    the same primitives.
    """

    def test_runs_on_synthetic_payload(self, tmp_path):
        from Cryptodome.Cipher import AES
        from Cryptodome.Util import Counter

        # Build a security_token that decrypts to a known key+nonce.
        master_key = base64.b64decode(
            "UIlTTEMmmLfGowo/UC60x2H45W6MdGgTRfo/umg4754="
        )
        track_key = b"0123456789abcdef"
        track_nonce = b"deadbeef"
        plain_st = track_key + track_nonce + b"\x00" * 8  # 32 bytes total

        iv = b"1234567890abcdef"
        aes_cbc = AES.new(master_key, AES.MODE_CBC, iv)
        encrypted_st = aes_cbc.encrypt(plain_st)
        security_token = iv + encrypted_st
        encryption_key_b64 = base64.b64encode(security_token).decode("ascii")

        # Build the encrypted "audio" payload using the same key/nonce.
        plaintext_audio = b"hello world this is a fake audio payload"
        ctr = Counter.new(64, prefix=track_nonce, initial_value=0)
        cipher = AES.new(track_key, AES.MODE_CTR, counter=ctr)
        encrypted_audio = cipher.encrypt(plaintext_audio)

        # Write encrypted file, decrypt, verify round-trip.
        enc_path = tmp_path / "encrypted.bin"
        dec_path = tmp_path / "decrypted.bin"
        enc_path.write_bytes(encrypted_audio)

        _decrypt_mqa(str(enc_path), str(dec_path), encryption_key_b64)

        assert dec_path.read_bytes() == plaintext_audio
