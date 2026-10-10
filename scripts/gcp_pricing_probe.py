#!/usr/bin/env python3
"""Read public regional disk SKU provenance without logging credentials."""
import argparse
import json
import urllib.request
import urllib.parse
from gcp_worker_image import token

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--credential", required=True)
    parser.add_argument("--region", required=True)
    args = parser.parse_args()
    with open(args.credential) as stream:
        access = token(json.load(stream), "https://www.googleapis.com/auth/cloud-platform")
    root = "https://cloudbilling.googleapis.com/v1/services/6F81-5844-456A/skus"
    page = ""
    while True:
        query = urllib.parse.urlencode({"currencyCode": "USD", "pageSize": 5000, "pageToken": page})
        request = urllib.request.Request(root+"?"+query, headers={"Authorization": "Bearer "+access})
        with urllib.request.urlopen(request, timeout=60) as response:
            result = json.load(response)
        for sku in result.get("skus", []):
            description = sku.get("description", "").lower()
            if args.region not in sku.get("serviceRegions", []):
                continue
            if "capacity" not in description or "pd" not in description:
                continue
            expression = sku.get("pricingInfo", [{}])[-1].get("pricingExpression", {})
            print(json.dumps({"skuId": sku.get("skuId"), "description": sku.get("description"),
                              "category": sku.get("category"), "usageUnit": expression.get("usageUnit"),
                              "rates": expression.get("tieredRates")}))
        page = result.get("nextPageToken", "")
        if not page:
            break

if __name__ == "__main__":
    main()
