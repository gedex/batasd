from pathlib import Path

message = Path("fixtures/message.txt").read_text().strip()
print(message)
