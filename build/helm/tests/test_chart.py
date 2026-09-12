"""Render the chart through Helm and verify deploy-critical configuration contracts."""

import base64
from pathlib import Path
import subprocess
import tempfile
import unittest

import yaml


CHART = Path(__file__).resolve().parents[1] / "thunderdome"


def helm_render(values=None):
    with tempfile.TemporaryDirectory() as directory:
        values_path = Path(directory) / "values.yaml"
        values_path.write_text(yaml.safe_dump(values or {}), encoding="utf-8")
        return subprocess.run(
            ["helm", "template", "thunderdome", str(CHART), "-f", str(values_path)],
            check=False, capture_output=True, text=True, encoding="utf-8",
        )


class ChartTest(unittest.TestCase):
    def render(self, values=None):
        result = helm_render(values)
        self.assertEqual(result.returncode, 0, result.stderr)
        return [doc for doc in yaml.safe_load_all(result.stdout) if doc]

    def one(self, documents, kind):
        matches = [doc for doc in documents if doc["kind"] == kind]
        self.assertEqual(len(matches), 1, kind)
        return matches[0]

    def container(self, documents, kind):
        return self.one(documents, kind)["spec"]["template"]["spec"]["containers"][0]

    def test_default_database_and_secret_wiring(self):
        docs = self.render()
        app = self.container(docs, "Deployment")
        pg = self.container(docs, "StatefulSet")
        secret = self.one(docs, "Secret")
        self.assertEqual(app["image"], "ghcr.io/jiayi-1994/thunderdome-planning-poker:latest")
        self.assertEqual(pg["image"], "docker.io/library/postgres:17-bookworm")
        keys = {env["name"]: env["valueFrom"]["secretKeyRef"] for env in app["env"]}
        self.assertEqual(set(keys), {"DB_PASS", "COOKIE_HASHKEY", "CONFIG_AES_HASHKEY"})
        for key, ref in keys.items():
            self.assertEqual(ref, {"name": secret["metadata"]["name"], "key": key})
            self.assertGreaterEqual(len(base64.b64decode(secret["data"][key])), 32)
        pg_password = next(env for env in pg["env"] if env["name"] == "POSTGRES_PASSWORD")
        self.assertEqual(pg_password["valueFrom"]["secretKeyRef"], keys["DB_PASS"])
        config = self.one(docs, "ConfigMap")["data"]
        self.assertEqual(config["DB_HOST"], "thunderdome-postgresql")
        self.assertEqual(config["COOKIE_SECURE"], "false")
        self.assertEqual(config["SMTP_ENABLED"], "false")
        self.assertEqual(config["HTTP_WRITE_TIMEOUT"], "60")

    def test_retains_database_and_credentials(self):
        docs = self.render()
        for kind in ("Secret", "PersistentVolumeClaim"):
            self.assertEqual(self.one(docs, kind)["metadata"]["annotations"]["helm.sh/resource-policy"], "keep")
        self.assertNotIn("storageClassName", self.one(docs, "PersistentVolumeClaim")["spec"])
        self.assertEqual(self.one(docs, "Deployment")["spec"]["strategy"]["type"], "Recreate")

    def test_proxy_inheritance_override_and_opt_out(self):
        docs = self.render({"global": {"imageProxy": "mirror.example.com/"}, "postgresql": {"image": {"proxy": ""}}})
        self.assertEqual(self.container(docs, "Deployment")["image"], "mirror.example.com/ghcr.io/jiayi-1994/thunderdome-planning-poker:latest")
        self.assertEqual(self.container(docs, "StatefulSet")["image"], "docker.io/library/postgres:17-bookworm")

    def test_domestic_example(self):
        docs = self.render(yaml.safe_load((CHART / "examples/values-cn.yaml").read_text()))
        self.assertEqual(self.container(docs, "Deployment")["image"], "ghcr.nju.edu.cn/jiayi-1994/thunderdome-planning-poker:latest")
        self.assertEqual(self.container(docs, "StatefulSet")["image"], "m.daocloud.io/docker.io/library/postgres:17-bookworm")

    def test_digest_overrides_tag(self):
        digest = "sha256:" + "a" * 64
        docs = self.render({"image": {"digest": digest}})
        self.assertEqual(self.container(docs, "Deployment")["image"], "ghcr.io/jiayi-1994/thunderdome-planning-poker@" + digest)

    def test_external_database_has_no_local_database_or_generated_secret(self):
        values = yaml.safe_load((CHART / "examples/values-external-db.yaml").read_text())
        docs = self.render(values)
        self.assertFalse({"StatefulSet", "PersistentVolumeClaim", "Secret"} & {d["kind"] for d in docs})
        config = self.one(docs, "ConfigMap")["data"]
        self.assertEqual(config["DB_HOST"], "postgres.example.com")
        self.assertEqual(config["DB_SSLMODE"], "require")

    def test_tls_and_prefix_are_consistent(self):
        docs = self.render({"app": {"domain": "poker.example.com", "pathPrefix": "/poker"}, "ingress": {"enabled": True, "tls": {"enabled": True, "secretName": "tls"}}})
        config = self.one(docs, "ConfigMap")["data"]
        self.assertEqual(config["COOKIE_SECURE"], "true")
        self.assertEqual(config["HTTP_SECURE_PROTOCOL"], "true")
        self.assertEqual(self.container(docs, "Deployment")["readinessProbe"]["httpGet"]["path"], "/poker/healthz")
        self.assertEqual(self.one(docs, "Ingress")["spec"]["rules"][0]["http"]["paths"][0]["path"], "/poker")

    def test_nodeport(self):
        docs = self.render({"postgresql": {"enabled": False}, "externalDatabase": {"host": "db"}, "secrets": {"existingSecret": "creds"}, "service": {"type": "NodePort", "nodePort": 30080}})
        self.assertEqual(self.one(docs, "Service")["spec"]["ports"][0]["nodePort"], 30080)

    def test_existing_and_static_storage(self):
        docs = self.render({"postgresql": {"persistence": {"existingClaim": "my-pvc"}}})
        self.assertNotIn("PersistentVolumeClaim", {doc["kind"] for doc in docs})
        volumes = self.one(docs, "StatefulSet")["spec"]["template"]["spec"]["volumes"]
        self.assertEqual(volumes[0]["persistentVolumeClaim"]["claimName"], "my-pvc")
        docs = self.render({"postgresql": {"persistence": {"storageClass": "-"}}})
        self.assertEqual(self.one(docs, "PersistentVolumeClaim")["spec"]["storageClassName"], "")

    def test_invalid_settings_fail_early(self):
        for values in (
            {"replicaCount": 2},
            {"app": {"domain": "https://poker.example.com"}},
            {"app": {"pathPrefix": "/poker/"}},
            {"ingress": {"enabled": True, "tls": {"enabled": True}}},
            {"postgresql": {"enabled": False}, "externalDatabase": {"host": "db"}},
            {"postgresql": {"enabled": False}, "secrets": {"existingSecret": "creds"}},
            {"secrets": {"cookieHashKey": "short"}},
        ):
            with self.subTest(values=values):
                self.assertNotEqual(helm_render(values).returncode, 0)


if __name__ == "__main__":
    unittest.main()
