import unittest
from gcp_worker_image import image_payload


class WorkerImageTest(unittest.TestCase):
    def vm(self):
        return {"name": "cloud-instance-builder", "status": "TERMINATED",
                "disks": [{"boot": True, "source": "https://www.googleapis.com/compute/v1/projects/p/zones/us-central1-a/disks/cloud-instance-builder"}]}

    def capture(self, vm):
        return image_payload("p", "us-central1-a", "cloud-instance-builder", vm, "akoflow-montage-docker-v1")

    def test_stopped_explicit_boot_disk(self):
        self.assertEqual(["us-central1"], self.capture(self.vm())["storageLocations"])

    def test_reject_running_wrong_identity_and_wrong_disk(self):
        for field, value in [("status", "RUNNING"), ("name", "other")]:
            vm = self.vm()
            vm[field] = value
            with self.assertRaises(ValueError):
                self.capture(vm)
        vm = self.vm()
        vm["disks"][0]["source"] = vm["disks"][0]["source"].replace("/projects/p/", "/projects/other/")
        with self.assertRaises(ValueError):
            self.capture(vm)


if __name__ == "__main__":
    unittest.main()
