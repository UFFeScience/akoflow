#!/usr/bin/env python3
"""Read-only proof of a managed worker's actual boot image, with no addresses."""
import argparse
import json
import re
import urllib.parse
import urllib.request
from gcp_worker_image import token

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--credential", required=True)
    parser.add_argument("--project", required=True)
    parser.add_argument("--zone", required=True)
    parser.add_argument("--image", required=True)
    parser.add_argument("instances", nargs="+")
    args = parser.parse_args()
    with open(args.credential) as stream:
        access = token(json.load(stream))
    root = "https://compute.googleapis.com/compute/v1/projects/"+urllib.parse.quote(args.project, safe="")
    expected = "https://www.googleapis.com/compute/v1/projects/"+args.project+"/global/images/"+args.image
    def get(endpoint):
        request = urllib.request.Request(endpoint, headers={"Authorization": "Bearer "+access})
        with urllib.request.urlopen(request, timeout=45) as response:
            return json.load(response)
    for instance in args.instances:
        if not re.fullmatch(r"cloud-instance-[a-z0-9-]+", instance):
            raise ValueError("An explicit managed instance is required")
        vm = get(root+"/zones/"+args.zone+"/instances/"+instance)
        disks = [disk for disk in vm.get("disks", []) if disk.get("boot")]
        if len(disks) != 1 or not disks[0]["source"].startswith("https://www.googleapis.com/compute/v1/projects/"+args.project+"/zones/"+args.zone+"/disks/"):
            raise ValueError("Unexpected boot disk identity")
        disk = get(disks[0]["source"])
        proof = {"instanceId": instance, "status": vm.get("status"),
                 "createdAt": vm.get("creationTimestamp"), "imageMatches": disk.get("sourceImage") == expected,
                 "sourceImageId": disk.get("sourceImageId"), "diskSizeGiB": disk.get("sizeGb")}
        print(json.dumps(proof))
        if not proof["imageMatches"]:
            raise ValueError("Boot disk does not match the requested immutable image")

if __name__ == "__main__":
    main()
