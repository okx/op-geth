"""Unit tests for report_pr_release.py (no network calls)."""
import base64
import hashlib
import hmac
import os
import sys
import unittest
from unittest.mock import MagicMock, patch

sys.path.insert(0, os.path.dirname(__file__))
import report_pr_release as rpr


class TestSign(unittest.TestCase):
    def test_known_vector(self):
        # Reproduce the example from the wiki:
        # secretKey="1234", GET /v1/logstores/logs, ts=1744359939894
        # Expected: /jgsYEgKRAwU3xILzGG4QWOQKJaegn1t6WCUDjuuuVo=
        sig = rpr._sign("GET", "/v1/logstores/logs", 1744359939894, "1234")
        self.assertEqual(sig, "/jgsYEgKRAwU3xILzGG4QWOQKJaegn1t6WCUDjuuuVo=")

    def test_signature_is_base64(self):
        sig = rpr._sign("POST", "/v1/change/jira-issue/report", 1000000000000, "secret")
        decoded = base64.b64decode(sig)
        self.assertEqual(len(decoded), 32)  # SHA-256 = 32 bytes


class TestJiraKeyValidation(unittest.TestCase):
    def test_valid_keys(self):
        valid = ["XLOP-1045", "DEVOPS-1234", "AUTO_PILOT-7", "PROD-12345"]
        for key in valid:
            self.assertRegex(key, rpr.JIRA_KEY_RE)

    def test_invalid_keys(self):
        invalid = ["xlop-1045", "1234-XLOP", "XLOP", "XLOP-", "-1045"]
        for key in invalid:
            self.assertNotRegex(key, rpr.JIRA_KEY_RE)

    def test_invalid_key_raises(self):
        os.environ["OKONE_APP_KEY"] = "ak"
        os.environ["OKONE_APP_SECRET"] = "sk"
        with self.assertRaises(ValueError):
            rpr.report_release("not-valid")


class TestExtractJiraKeys(unittest.TestCase):
    def test_extracts_single_key(self):
        log = "feat: something\n\nJira: XLOP-1045\n"
        keys = rpr._extract_jira_keys_from_log(log)
        self.assertEqual(keys, ["XLOP-1045"])

    def test_deduplicates_keys(self):
        log = "Jira: XLOP-1045\nJira: XLOP-1045\n"
        keys = rpr._extract_jira_keys_from_log(log)
        self.assertEqual(len(keys), 1)

    def test_multiple_keys(self):
        log = "Jira: XLOP-1045\nJira: DEVOPS-999\n"
        keys = rpr._extract_jira_keys_from_log(log)
        self.assertIn("XLOP-1045", keys)
        self.assertIn("DEVOPS-999", keys)

    def test_no_jira_key(self):
        log = "fix: typo\n"
        keys = rpr._extract_jira_keys_from_log(log)
        self.assertEqual(keys, [])


class TestReportRelease(unittest.TestCase):
    def setUp(self):
        os.environ["OKONE_APP_KEY"] = "test_ak"
        os.environ["OKONE_APP_SECRET"] = "test_sk"
        os.environ["OKONE_API_BASE_URL"] = "https://example.com/apigw"

    def tearDown(self):
        for k in ("OKONE_APP_KEY", "OKONE_APP_SECRET", "OKONE_API_BASE_URL", "OKONE_AUTO_CLOSE"):
            os.environ.pop(k, None)

    def test_missing_credentials_raises(self):
        del os.environ["OKONE_APP_KEY"]
        del os.environ["OKONE_APP_SECRET"]
        with self.assertRaises(RuntimeError, msg="OKONE_APP_KEY and OKONE_APP_SECRET must be set"):
            rpr.report_release("XLOP-1045")

    @patch("report_pr_release.request.urlopen")
    def test_successful_call(self, mock_urlopen):
        mock_response = MagicMock()
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)
        mock_response.read.return_value = b'{"code": 0, "msg": "", "data": {"id": 1}}'
        mock_urlopen.return_value = mock_response

        result = rpr.report_release("XLOP-1045")

        self.assertEqual(result["code"], 0)
        call_args = mock_urlopen.call_args
        req = call_args[0][0]
        # urllib.Request stores headers with title-cased keys
        self.assertIn("X-app-key", req.headers)
        self.assertIn("X-timestamp", req.headers)
        self.assertIn("X-signature", req.headers)
        self.assertEqual(req.get_header("Content-type"), "application/json")

    @patch("report_pr_release.request.urlopen")
    def test_api_error_raises(self, mock_urlopen):
        mock_response = MagicMock()
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)
        mock_response.read.return_value = b'{"code": 1001, "msg": "not found"}'
        mock_urlopen.return_value = mock_response

        with self.assertRaises(RuntimeError):
            rpr.report_release("XLOP-1045")

    @patch("report_pr_release.request.urlopen")
    def test_auto_close_flag(self, mock_urlopen):
        import json as _json
        mock_response = MagicMock()
        mock_response.__enter__ = lambda s: s
        mock_response.__exit__ = MagicMock(return_value=False)
        mock_response.read.return_value = b'{"code": 0, "msg": "", "data": {}}'
        mock_urlopen.return_value = mock_response

        os.environ["OKONE_AUTO_CLOSE"] = "true"
        rpr.report_release("XLOP-1045")

        req = mock_urlopen.call_args[0][0]
        body = _json.loads(req.data)
        self.assertTrue(body["autoCloseJiraIssue"])


class TestMainCLI(unittest.TestCase):
    def test_no_args_returns_1(self):
        result = rpr.main(["script"])
        self.assertEqual(result, 1)

    @patch("report_pr_release.report_release")
    def test_success_returns_0(self, mock_report):
        mock_report.return_value = {"code": 0}
        result = rpr.main(["script", "XLOP-1045"])
        self.assertEqual(result, 0)

    @patch("report_pr_release.report_release")
    def test_error_returns_1(self, mock_report):
        mock_report.side_effect = RuntimeError("boom")
        result = rpr.main(["script", "XLOP-1045"])
        self.assertEqual(result, 1)


if __name__ == "__main__":
    unittest.main()
