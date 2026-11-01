# Excommunicado

Scores your mail from 0 to 10 for corporate bullshit, in English and Swedish. An Outlook add-in shows the score next to the mail. A small language model does the work on your own PC, so mail text does not leave it.

## How it works

```mermaid
flowchart LR
  O[Outlook add-in] -->|mail text over HTTPS on localhost| A[app container]
  A --> L[llama container, no internet access]
```

The score comes from two sources:

- **Language model:** [Qwen3.5 0.8B](https://huggingface.co/bartowski/Qwen_Qwen3.5-0.8B-GGUF) (Q4_K_M) running in llama.cpp.
- **Word list:** English and Swedish buzzwords in `internal/scorer/lexicon.go`. It sets a minimum score and supplies most of the words the pane shows.

Quoted reply history and links are removed before scoring, so you score what the sender wrote. The model reads the first 2400 characters; the word list reads the whole mail.

## Install on Windows

You need Podman or Docker with `compose`. Both run containers in a small Linux VM (WSL2).

```powershell
powershell -ExecutionPolicy Bypass -File scripts\setup.ps1
```

The script downloads the x86 images from GitHub, starts the containers, then installs the HTTPS certificate for your Windows user. Windows asks once whether to trust it: answer Yes. No admin rights are needed for this step. Then add the add-in:

1. Open <https://aka.ms/olksideload> and sign in.
2. Choose **Custom Addins > Add a custom add-in > Add from File**.
3. Choose `internal\server\web\manifest.xml` from this repo.
4. Open a mail and choose **Apps** (or the ribbon) **> Bullshit score**. Pin the pane to keep it open while you move between mails.

The containers start again by themselves when the container runtime starts. With Podman on Windows, the Podman machine must be running first (`podman machine start`).

## Try it without Outlook

Works on macOS, Linux and Windows:

```sh
make up        # builds the images locally, or: podman compose up -d --build
```

Open <https://localhost:47823/taskpane.html> and paste a mail. Your browser warns about the certificate until you trust it. On macOS:

```sh
curl -sk https://localhost:47823/cert.pem -o /tmp/excommunicado.pem
security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db /tmp/excommunicado.pem
```

Or use the API:

```sh
curl -sk https://localhost:47823/api/score -H 'Content-Type: application/json' \
  -d '{"subject":"","body":"Vi ska kraftsamla och skapa synergier på vår förändringsresa."}'
```

## Memory

The containers are limited to 950 MB (model) and 50 MB (app). Measured peak: about 840 MB and 12 MB. The VM that runs the containers on Windows uses extra memory on top of this.

## Privacy and security

- The port is published on `127.0.0.1` only. Requests that name another host are refused.
- The model container has no internet access.
- The server does not log mail text.
- `office.js` must load from Microsoft's server when the pane opens. This is a Microsoft rule. The pane sends no mail text there.
- The certificate is self-signed, is not a CA, and is valid for localhost only.

## Limits

- It scores one mail at a time: the one you have open. Scoring a whole mailbox would need a Microsoft Graph app registration, which usually needs admin approval.
- Your organization can block custom add-ins. If so, the paste page above still works.
- The model is small and sometimes wrong, mostly on jargon that is not in the word list, and more often in Swedish. To improve the score for your workplace, add its favorite words to `lexicon.go`.
- A mail can talk the model into a higher score, up to 5 of 10 unless the mail has real buzzwords. It can't push the score below what the word list finds.

## Troubleshooting

| Problem | Fix |
| --- | --- |
| The pane is blank | Open <https://localhost:47823/taskpane.html> in Edge. If it shows a certificate warning, run `setup.ps1` again and restart Outlook. |
| "The scoring service is not answering" | Check `podman compose ps` and `podman compose logs llama`. The model needs a few seconds to load after a start. |
| Certificate stops working | It is renewed after about two years, or when the `certs` volume is deleted. Run `setup.ps1` again. |

## Development

```sh
make test vet build
make race
```

Pushing a `v*` tag runs the tests, then builds, scans and publishes the x86 images to `ghcr.io/mnorrsken/excommunicado` and `ghcr.io/mnorrsken/excommunicado-llama`.

## License

Apache 2.0, see [LICENSE](LICENSE). The model is also Apache 2.0.
