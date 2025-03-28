## Go HTTP Audio Server

This is a rudimentary Go HTTP server for recording, transcribing, and translating audio.

### Features

*   Start audio recording using `pw-record`.
*   Stop audio recording.
*   Transcribe audio using the OpenAI Whisper API.
*   Translate audio using the OpenAI ChatCompletion API.

### Requirements

*   Linux
*   Go
*   `pw-record` (PipeWire audio recorder)
*   OpenAI API Key (for Whisper and ChatCompletion models)

### Installation

1.  Clone the repository.
2.  Install dependencies:

    ```bash
    go mod tidy
    ```

3.  Set up your environment variables:

    *   Create a `.env` file in the project root.
    *   Add your OpenAI API keys to the `.env` file:

        ```
        OPENAI_WHISPER_API_KEY=<YOUR_OPENAI_WHISPER_API_KEY>
        OPENAI_TRANSCRIBE_API_KEY=<YOUR_OPENAI_CHATCOMPLETION_API_KEY>
        ```

### Usage

1.  Run the server:
    ```bash
    go run main.go
    ```
    The server will start on port `5757`.
2.  API Endpoints:
    *   `POST /audio/start`: Starts audio recording.
      *   Response:
          *   ```json
              {"id": "<process_id>", "message": "Recording started", "file": "<output_file>"}
              ```
    *   `POST /audio/stop?id=<process_id>`: Stops audio recording.
        *   Query Parameter:
            *   `id`: The process ID returned by `/audio/start`.
        *   Response:
          *   ```json
              {"message": "Recording stopped", "file": "<file_name>"}
              ```
    *   `POST /audio/transcribe?filename=<filename>&lang=<language_code>`: Transcribes audio using OpenAI Whisper.
        *   Query Parameters:
            *   `filename`: The path to the audio file to transcribe.
            *   `lang` (Optional): The language of the audio (e.g., `en` for English, `ru` for Russian). If the language is not valid, it will be ignored.
        *   Response:
          *   ```json
              {"message": "<transcription_text>"}
              ```
    *   `POST /audio/translate?input=<input_text>&prompt=<prompt_text>`: Translates audio using OpenAI ChatCompletion.
        *   Query Parameters:
            *   `input`: The input text to translate.
            *   `prompt`: The prompt to guide the translation model.
        *   Response:
          *   ```json
              {"message": "<translation_text>"}
              ```
### CORS Configuration

The server is configured to allow requests from `http://localhost:3000`. You can adjust the `AllowOrigins` configuration in `main.go` if needed.

### Notes

*   The server uses `pw-record` for audio recording, ensure it is correctly configured in your system.
*   The `removeTempFiles` function is currently not working as expected and may require adjustments.
*   Error handling is rudimentary and logging could be improved.

### License

[MIT](LICENSE)
