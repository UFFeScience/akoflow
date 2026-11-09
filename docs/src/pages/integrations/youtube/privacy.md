---
title: Video Publisher Privacy Policy
description: How the AkôFlow video publisher accesses and handles YouTube data.
---

# Video Publisher Privacy Policy

Effective date: November 8, 2026.

This policy applies to Akoflow Video - homeserver, the operator-managed AkôFlow Video Publisher. It describes this publishing integration, rather than every AkôFlow deployment.

## Data accessed and purpose

With the channel operator's consent, the publisher uses Google's OAuth service and the YouTube Data API to identify the authorized channel, upload operator-selected videos and metadata, create or locate the AkôFlow Guide playlist, manage its video membership, and upload English captions.

The requested permission is `youtube.force-ssl`. Google describes this permission as allowing access to view, edit, and permanently delete YouTube videos, ratings, comments, and captions. The publisher uses this permission for the publishing functions above; the workflow does not collect ratings or comments. Any removal of an incomplete upload requires the operator's authorization.

## Storage and sharing

OAuth client credentials and refresh tokens are stored in a private configuration directory on the operator's homeserver, accessible only to the operating-system account that runs the publisher. They are kept outside source repositories, public media directories, and the website. They are used to authenticate requests to Google.

Selected video files, captions, titles, descriptions, and playlist metadata are sent to Google to perform the requested publication. Local publishing receipts contain file hashes, video and playlist identifiers, and verification results. These materials remain available to the operator for verification and avoiding duplicate uploads.

The publishing workflow does not sell Google user data, use it for advertising, provide it to data brokers, or use it to train generalized AI models. The publisher's use and transfer of information received from Google APIs adheres to the [Google API Services User Data Policy](https://developers.google.com/terms/api-services-user-data-policy), including its Limited Use requirements.

## Visibility and retention

New videos and playlists are unlisted by default. Anyone with an unlisted link can view and share it; unlisted does not mean private. Uploaded content is retained on YouTube until removed by an authorized channel operator, subject to Google's policies. Local source media and publishing receipts remain under the operator's control until the operator removes them.

Credentials are retained to support future authorized publishing and can be removed from the homeserver by its operator. Removing a local token does not remove videos already uploaded to YouTube.

## Revocation and deletion

You can revoke this app's access at any time in [Google Account connections](https://myaccount.google.com/connections). Revocation prevents future API access with that authorization. Channel operators can manage or delete uploaded videos and playlists in [YouTube Studio](https://studio.youtube.com/).

To request removal of locally stored integration data or ask about this policy, contact [ovvesley@gmail.com](mailto:ovvesley@gmail.com). Requests are handled by the operator; there is no public account-registration service for this publisher.

Google processes uploaded content and account data under its [Privacy Policy](https://policies.google.com/privacy).
