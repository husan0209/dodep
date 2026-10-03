"""Tests for the Redpanda consumer's threading model.

The poll loop runs in an executor thread while handlers are async, so event
dispatch must hop back onto the event loop (a bare `await` inside the poll
thread was a SyntaxError: 'await' outside async function).
"""

import asyncio
import json

import pytest

from internal.consumers.redpanda_consumer import RedpandaConsumer

# This module exercises async handlers directly.
pytestmark = pytest.mark.asyncio


class StubPrediction:
    is_fraud = True
    user_id = 7
    fraud_type = "account_takeover"
    risk_score = 91.0
    explanation = "new device + new country"


class StubDetector:
    def __init__(self):
        self.calls = []

    def detect_account_takeover(self, user_id, login_data):
        self.calls.append((user_id, login_data))
        return StubPrediction()


class StubMessage:
    def __init__(self, topic="users.logins", payload=None, error=None):
        self._topic = topic
        self._payload = payload
        self._error = error

    def error(self):
        return self._error

    def topic(self):
        return self._topic

    def value(self):
        return json.dumps(self._payload).encode("utf-8") if self._payload else b""


class StubConsumer:
    """Replays a scripted list of messages, then stops the loop."""

    def __init__(self, messages):
        self._messages = list(messages)
        self.subscribed = []
        self.closed = False

    def subscribe(self, topics):
        self.subscribed.append(topics)

    def poll(self, timeout=1.0):
        if self._messages:
            return self._messages.pop(0)
        # Idle: signal the loop to exit by clearing `running`.
        return None

    def close(self):
        self.closed = True


LOGIN_EVENT = {
    "user_id": 7,
    "ip": "203.0.113.5",
    "country": "NL",
    "device_fingerprint": "dev-xyz",
}


def make_consumer(monkeypatch, messages, detector=None):
    """Build a consumer whose Kafka client is replaced by a stub."""
    consumer = RedpandaConsumer(
        brokers=["localhost:9092"],
        fraud_detector=detector or StubDetector(),
    )
    stub = StubConsumer(messages)

    def fake_client(config):
        stub.config = config
        return stub

    monkeypatch.setattr("internal.consumers.redpanda_consumer.Consumer", fake_client)
    return consumer, stub


class TestRedpandaConsumer:
    async def test_start_subscribes_and_consumes(self, monkeypatch):
        messages = [StubMessage(payload=LOGIN_EVENT)]
        consumer, stub = make_consumer(monkeypatch, messages)
        detector = consumer.fraud_detector

        await consumer.start()
        # Wait for the executor thread to drain the scripted messages.
        for _ in range(200):
            await asyncio.sleep(0.01)
            if not messages:
                break

        await consumer.stop()

        assert stub.subscribed == [consumer.topics]
        assert consumer.stats["messages_consumed"] >= 1
        # Async handler ran on the event loop and reached the detector.
        assert len(detector.calls) == 1
        assert detector.calls[0][0] == 7

    async def test_malformed_payload_is_counted_as_error(self, monkeypatch):
        bad = StubMessage()
        bad._payload = None
        bad.value = lambda: b"not-json"  # type: ignore[method-assign]
        consumer, _ = make_consumer(monkeypatch, [bad])

        await consumer.start()
        for _ in range(200):
            await asyncio.sleep(0.01)
            if consumer.stats["errors"] > 0:
                break
        await consumer.stop()

        assert consumer.stats["errors"] >= 1
        assert consumer.stats["messages_consumed"] >= 1

    async def test_unknown_topic_is_ignored(self, monkeypatch):
        consumer, _ = make_consumer(
            monkeypatch, [StubMessage(topic="unknown.topic", payload=LOGIN_EVENT)]
        )

        await consumer.start()
        for _ in range(200):
            await asyncio.sleep(0.01)
            if consumer.stats["messages_consumed"] > 0:
                break
        await consumer.stop()

        assert consumer.stats["messages_consumed"] >= 1
        assert consumer.stats["errors"] == 0

    async def test_handler_error_is_isolated(self, monkeypatch):
        class ExplodingDetector(StubDetector):
            def detect_account_takeover(self, user_id, login_data):
                raise RuntimeError("detector exploded")

        consumer, _ = make_consumer(
            monkeypatch, [StubMessage(payload=LOGIN_EVENT)], detector=ExplodingDetector()
        )

        await consumer.start()
        for _ in range(200):
            await asyncio.sleep(0.01)
            if consumer.stats["errors"] > 0:
                break
        await consumer.stop()

        # One bad event must not kill the consumer loop.
        assert consumer.stats["errors"] >= 1

    async def test_dispatch_without_loop_is_noop(self, monkeypatch):
        """A late message after shutdown must not raise."""
        consumer, _ = make_consumer(monkeypatch, [])
        consumer._loop = None
        consumer._dispatch("users.logins", LOGIN_EVENT)  # must not raise

    async def test_dispatch_uses_scheduled_coroutine(self, monkeypatch):
        """Dispatch hops from the poll thread onto the event loop.

        Called from a real worker thread (as `_consume_loop` does) — calling
        it from the loop thread itself would block on `future.result()` while
        the coroutine waits for that same loop, which is exactly the deadlock
        the threadsafe scheduling avoids.
        """
        consumer, _ = make_consumer(monkeypatch, [])
        seen = {}

        async def record(topic, event):
            seen["topic"] = topic
            seen["event"] = event

        consumer._loop = asyncio.get_running_loop()
        consumer._process_event = record  # type: ignore[assignment]

        await asyncio.to_thread(consumer._dispatch, "users.logins", LOGIN_EVENT)

        assert seen == {"topic": "users.logins", "event": LOGIN_EVENT}
        assert consumer.stats["errors"] == 0

    async def test_stop_closes_client(self, monkeypatch):
        consumer, stub = make_consumer(monkeypatch, [])
        await consumer.start()
        await consumer.stop()

        assert stub.closed is True
        assert consumer.consumer is None
        assert consumer.running is False

    async def test_get_stats_is_a_copy(self, monkeypatch):
        consumer, _ = make_consumer(monkeypatch, [])
        stats = consumer.get_stats()
        stats["errors"] = 999
        assert consumer.stats["errors"] == 0
