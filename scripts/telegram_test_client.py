#!/usr/bin/env python3
"""
Telegram Test Client using Telethon.
Logs in as a Telegram user, sends a message to a bot, and waits for a response.
Loads configuration automatically from the project's root .env file.
"""
import argparse
import asyncio
import os
import sys

try:
    from telethon import TelegramClient, events
except ImportError:
    print("Error: The 'telethon' library is not installed.", file=sys.stderr)
    print("Please install it using: pip install telethon", file=sys.stderr)
    sys.exit(1)


async def main():
    # Load .env file manually from the project root directory
    script_dir = os.path.dirname(os.path.abspath(__file__))
    env_path = os.path.join(os.path.dirname(script_dir), ".env")
    if os.path.exists(env_path):
        with open(env_path) as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith("#") and "=" in line:
                    key, val = line.split("=", 1)
                    os.environ[key.strip()] = val.strip()

    parser = argparse.ArgumentParser(
        description="Telethon-based Telegram client to send a message to a bot and wait for responses."
    )
    parser.add_argument(
        "--api-id",
        type=int,
        default=os.getenv("TELEGRAM_API_ID"),
        help="Telegram API ID (integer). Can also be set via TELEGRAM_API_ID env var."
    )
    parser.add_argument(
        "--api-hash",
        default=os.getenv("TELEGRAM_API_HASH"),
        help="Telegram API Hash. Can also be set via TELEGRAM_API_HASH env var."
    )
    parser.add_argument(
        "--phone",
        default=os.getenv("TELEGRAM_PHONE") or os.getenv("TELEGRAM_TEST_PHONE"),
        help="Phone number with country code (e.g., +123456789). Can also be set via TELEGRAM_PHONE or TELEGRAM_TEST_PHONE env var."
    )
    parser.add_argument(
        "--bot",
        default=os.getenv("TELEGRAM_BOT_USERNAME"),
        help="Username of the bot to test (e.g. @MyBot). Can also be set via TELEGRAM_BOT_USERNAME env var."
    )
    parser.add_argument(
        "--message",
        default="Ping",
        help="The message text to send to the bot."
    )
    parser.add_argument(
        "--timeout",
        type=float,
        default=10.0,
        help="Timeout in seconds to wait for a response."
    )
    parser.add_argument(
        "--session",
        default=os.path.join(script_dir, "telegram_test_session"),
        help="Filename/path for the Telethon session file."
    )

    args = parser.parse_args()

    if not args.api_id or not args.api_hash:
        print("Error: Both --api-id and --api-hash are required.", file=sys.stderr)
        print("Set them via command-line arguments or environment variables:", file=sys.stderr)
        print("  export TELEGRAM_API_ID=123456", file=sys.stderr)
        print("  export TELEGRAM_API_HASH=abcdef123456...", file=sys.stderr)
        sys.exit(1)

    phone_number = args.phone
    if not phone_number:
        # Request phone number if not supplied
        phone_number = input("Enter your phone number (with country code, e.g., +123456789): ").strip()
        if not phone_number:
            print("Error: Phone number is required to log in.", file=sys.stderr)
            sys.exit(1)

    if not args.bot:
        print("Error: Bot username is required (--bot or TELEGRAM_BOT_USERNAME).", file=sys.stderr)
        sys.exit(1)

    bot_username = args.bot if args.bot.startswith("@") else f"@{args.bot}"

    print(f"Connecting to Telegram and initializing session '{args.session}'...")
    client = TelegramClient(args.session, args.api_id, args.api_hash)

    # Start the client. This will handle login interactively via stdin/stdout if needed.
    await client.start(phone=phone_number)
    print("Successfully connected and authenticated!")

    # Verify bot entity exists
    try:
        bot_entity = await client.get_input_entity(bot_username)
    except Exception as e:
        print(f"Error: Could not find bot with username '{bot_username}'. Details: {e}", file=sys.stderr)
        await client.disconnect()
        sys.exit(1)

    print(f"Sending message to {bot_username}: '{args.message}'")
    
    # Event handler for incoming responses
    response_received = asyncio.Event()
    responses = []

    @client.on(events.NewMessage(incoming=True, chats=bot_entity))
    async def handler(event):
        print(f"\n--- Bot Response Received ---")
        print(event.text)
        print("-----------------------------\n")
        responses.append(event.text)
        response_received.set()

    # Send the message
    await client.send_message(bot_entity, args.message)

    print(f"Waiting up to {args.timeout} seconds for responses from {bot_username}...")
    try:
        # Wait for the first response
        await asyncio.wait_for(response_received.wait(), timeout=args.timeout)
        # Give a small window (1 second) to catch any additional messages the bot sends in response
        await asyncio.sleep(1.0)
        print(f"Test completed. Received {len(responses)} response(s).")
    except asyncio.TimeoutError:
        print(f"Timeout: No response received from {bot_username} within {args.timeout} seconds.", file=sys.stderr)
        await client.disconnect()
        sys.exit(1)

    await client.disconnect()
    print("Disconnected.")

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\nOperation cancelled by user.")
        sys.exit(1)
