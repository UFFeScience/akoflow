#!/usr/bin/env python3
"""Capture a stopped, disposable worker's boot disk; never print credentials.

Run in the server container (cryptography is supplied by its Ansible runtime).
VM configuration, stopping and destruction remain Akoflow API operations.
"""
import argparse
import base64
import json
import re
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid


def image_payload(project, zone, instance, vm, image_name):
    if not re.fullmatch(r"cloud-instance-[a-z0-9-]+", instance):
        raise ValueError("Expected an explicit managed builder instance")
    if not re.fullmatch(r"akoflow-montage-docker-[a-z0-9-]+", image_name):
        raise ValueError("Expected a versioned Montage worker image name")
    if vm.get("name") != instance or vm.get("status") != "TERMINATED":
        raise ValueError("Builder identity must match and VM must be stopped")
    disks = [d for d in vm.get("disks", []) if d.get("boot")]
    if len(disks) != 1:
        raise ValueError("Builder must have exactly one boot disk")
    source = disks[0].get("source", "")
    expected = f"https://www.googleapis.com/compute/v1/projects/{project}/zones/{zone}/disks/"
    if not source.startswith(expected) or not source.removeprefix(expected).startswith(instance):
        raise ValueError("Boot disk must belong to the explicit builder and zone")
    return {"name": image_name, "sourceDisk": source,
            "description": "AKO-136 Docker-ready Montage worker; sanitized disposable builder",
            "labels": {"managed-by": "akoflow", "purpose": "montage-worker", "issue": "ako-136"},
            "storageLocations": [zone.rsplit("-", 1)[0]]}


def token(credential):
    from cryptography.hazmat.primitives import hashes, serialization
    from cryptography.hazmat.primitives.asymmetric import padding
    def b64(data):
        return base64.urlsafe_b64encode(data).rstrip(b"=")
    now = int(time.time())
    claims = {"iss": credential["client_email"], "aud": credential["token_uri"],
              "scope": "https://www.googleapis.com/auth/compute", "iat": now, "exp": now + 3600}
    unsigned = b64(b'{"alg":"RS256","typ":"JWT"}') + b"." + b64(json.dumps(claims).encode())
    key = serialization.load_pem_private_key(credential["private_key"].encode(), None)
    assertion = unsigned + b"." + b64(key.sign(unsigned, padding.PKCS1v15(), hashes.SHA256()))
    body = urllib.parse.urlencode({"grant_type": "urn:ietf:params:oauth:grant-type:jwt-bearer",
                                   "assertion": assertion.decode()}).encode()
    request = urllib.request.Request(credential["token_uri"], data=body)
    with urllib.request.urlopen(request, timeout=45) as response:
        return json.load(response)["access_token"]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["create", "status"])
    parser.add_argument("--credential", required=True)
    parser.add_argument("--project", required=True)
    parser.add_argument("--zone", required=True)
    parser.add_argument("--instance", required=True)
    parser.add_argument("--image", required=True)
    args = parser.parse_args()
    with open(args.credential) as stream:
        credential = json.load(stream)
    access = token(credential)
    root = "https://compute.googleapis.com/compute/v1/projects/" + urllib.parse.quote(args.project, safe="")
    def call(path, body=None):
        headers = {"Authorization": "Bearer " + access, "Content-Type": "application/json"}
        request = urllib.request.Request(root + path, headers=headers,
                                         data=None if body is None else json.dumps(body).encode())
        with urllib.request.urlopen(request, timeout=45) as response:
            return json.load(response)
    path = "/global/images/" + urllib.parse.quote(args.image, safe="")
    if args.action == "status":
        image = call(path)
        print(json.dumps({k: image.get(k) for k in ("name", "status", "selfLink", "creationTimestamp", "diskSizeGb")}))
        return
    vm = call("/zones/" + args.zone + "/instances/" + args.instance)
    body = image_payload(args.project, args.zone, args.instance, vm, args.image)
    try:
        existing = call(path)
    except urllib.error.HTTPError as error:
        if error.code != 404:
            raise
    else:
        if existing.get("sourceDisk") != body["sourceDisk"] or existing.get("labels", {}).get("issue") != "ako-136":
            raise ValueError("Existing image has a different source/ownership; refusing overwrite")
        print(json.dumps({"name": existing["name"], "status": existing["status"], "reused": True}))
        return
    request_id = uuid.uuid5(uuid.NAMESPACE_URL, root + path)
    operation = call("/global/images?requestId=" + str(request_id), body)
    print(json.dumps({"operation": operation.get("name"), "status": operation.get("status"), "image": args.image}))


if __name__ == "__main__":
    try:
        main()
    except urllib.error.HTTPError as error:
        raise SystemExit("GCP request failed with HTTP " + str(error.code))
