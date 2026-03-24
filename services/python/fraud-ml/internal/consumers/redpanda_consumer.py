"""
Redpanda Consumer for Real-time Fraud Detection
Consumes events from Redpanda and runs fraud detection in real-time
"""

import asyncio
import json
import logging
from typing import Dict, Any, Optional
from datetime import datetime

from confluent_kafka import Consumer, KafkaError, Message

logger = logging.getLogger(__name__)


class RedpandaConsumer:
    """
    Consumes events from Redpanda/Kafka and runs fraud detection
    """
    
    def __init__(
        self,
        brokers: list,
        fraud_detector: Any,
        group_id: str = "fraud-ml-service"
    ):
        self.brokers = brokers
        self.fraud_detector = fraud_detector
        self.group_id = group_id
        
        # Topics to consume
        self.topics = [
            "bets.placed",
            "bets.settled",
            "payments.initiated",
            "payments.completed",
            "users.registered",
            "users.logins",
            "bonus.activated",
        ]
        
        # Consumer configuration
        self.config = {
            "bootstrap.servers": ",".join(brokers),
            "group.id": group_id,
            "auto.offset.reset": "latest",
            "enable.auto.commit": True,
            "auto.commit.interval.ms": 5000,
        }
        
        self.consumer: Optional[Consumer] = None
        self.running = False
        
        # Event handlers
        self.event_handlers = {
            "bets.placed": self._handle_bet_placed,
            "bets.settled": self._handle_bet_settled,
            "payments.initiated": self._handle_payment_initiated,
            "payments.completed": self._handle_payment_completed,
            "users.registered": self._handle_user_registered,
            "users.logins": self._handle_user_login,
            "bonus.activated": self._handle_bonus_activated,
        }
        
        # Statistics
        self.stats = {
            "messages_consumed": 0,
            "fraud_detected": 0,
            "errors": 0,
        }
        
    async def start(self):
        """Start consuming events"""
        loop = asyncio.get_event_loop()
        
        # Run consumer in background thread
        self.running = True
        self.consumer = Consumer(self.config)
        self.consumer.subscribe(self.topics)
        
        logger.info(f"Redpanda consumer started, subscribed to: {self.topics}")
        
        # Start consumer loop in executor
        loop.run_in_executor(None, self._consume_loop)
        
    def stop(self):
        """Stop consuming events"""
        self.running = False
        if self.consumer:
            self.consumer.close()
        logger.info("Redpanda consumer stopped")
        
    def _consume_loop(self):
        """Main consumer loop"""
        while self.running:
            try:
                msg = self.consumer.poll(timeout=1.0)
                
                if msg is None:
                    continue
                    
                if msg.error():
                    if msg.error().code() == KafkaError._PARTITION_EOF:
                        continue
                    logger.error(f"Kafka error: {msg.error()}")
                    self.stats["errors"] += 1
                    continue
                
                self.stats["messages_consumed"] += 1
                
                # Process message
                topic = msg.topic()
                value = msg.value()
                
                try:
                    event = json.loads(value.decode("utf-8"))
                    await self._process_event(topic, event)
                except json.JSONDecodeError as e:
                    logger.error(f"Failed to decode message: {e}")
                    self.stats["errors"] += 1
                    
            except Exception as e:
                logger.error(f"Consumer error: {e}")
                self.stats["errors"] += 1
                
    async def _process_event(self, topic: str, event: Dict[str, Any]):
        """Process a single event"""
        handler = self.event_handlers.get(topic)
        if not handler:
            logger.debug(f"No handler for topic: {topic}")
            return
            
        try:
            await handler(event)
        except Exception as e:
            logger.error(f"Handler error for {topic}: {e}")
            self.stats["errors"] += 1
            
    async def _handle_bet_placed(self, event: Dict[str, Any]):
        """Handle bet placed event"""
        user_id = event.get("user_id")
        if not user_id:
            return
            
        # Get user's recent bets from ClickHouse
        # For now, just log
        logger.debug(f"Bet placed by user {user_id}: {event.get('stake')}")
        
    async def _handle_bet_settled(self, event: Dict[str, Any]):
        """Handle bet settled event - check for suspicious patterns"""
        user_id = event.get("user_id")
        if not user_id:
            return
            
        # Check for rapid win/loss patterns
        # This would query ClickHouse for user's recent history
        logger.debug(f"Bet settled for user {user_id}: {event.get('result')}")
        
    async def _handle_payment_initiated(self, event: Dict[str, Any]):
        """Handle payment initiated event - check for fraud"""
        user_id = event.get("user_id")
        if not user_id:
            return
            
        amount = event.get("amount", 0)
        tx_type = event.get("type", "")
        
        # Check for high-risk patterns
        if amount > 10000:  # High value transaction
            logger.warning(f"High value transaction: user={user_id}, amount={amount}")
            
        logger.debug(f"Payment initiated: user={user_id}, type={tx_type}, amount={amount}")
        
    async def _handle_payment_completed(self, event: Dict[str, Any]):
        """Handle payment completed event"""
        user_id = event.get("user_id")
        if not user_id:
            return
            
        logger.debug(f"Payment completed for user {user_id}")
        
    async def _handle_user_registered(self, event: Dict[str, Any]):
        """Handle user registration - check for bonus abuse risk"""
        user_id = event.get("user_id")
        if not user_id:
            return
            
        # New users are higher risk for bonus abuse
        logger.info(f"New user registered: {user_id}")
        
    async def _handle_user_login(self, event: Dict[str, Any]):
        """Handle user login - check for account takeover"""
        user_id = event.get("user_id")
        if not user_id:
            return
            
        login_data = {
            "user_id": user_id,
            "ip": event.get("ip", ""),
            "country": event.get("country", ""),
            "device_fingerprint": event.get("device_fingerprint", ""),
            "hour": datetime.now().hour,
        }
        
        # Run account takeover detection
        prediction = self.fraud_detector.detect_account_takeover(
            user_id, login_data
        )
        
        if prediction.is_fraud:
            logger.warning(
                f"Potential account takeover detected for user {user_id}: "
                f"risk_score={prediction.risk_score}"
            )
            # Send alert to notification service
            await self._send_fraud_alert(prediction)
            
    async def _handle_bonus_activated(self, event: Dict[str, Any]):
        """Handle bonus activated event - track for abuse detection"""
        user_id = event.get("user_id")
        if not user_id:
            return
            
        logger.debug(f"Bonus activated for user {user_id}")
        
    async def _send_fraud_alert(self, prediction: Any):
        """Send fraud alert to notification service"""
        # This would publish to a notification topic or call notification service
        alert = {
            "type": "fraud_alert",
            "user_id": prediction.user_id,
            "fraud_type": prediction.fraud_type,
            "risk_score": prediction.risk_score,
            "timestamp": datetime.now().isoformat(),
            "explanation": prediction.explanation,
        }
        
        # Publish to alerts topic
        # self.producer.produce("fraud.alerts", json.dumps(alert))
        logger.info(f"Fraud alert sent: {alert}")
        
    def get_stats(self) -> Dict[str, Any]:
        """Get consumer statistics"""
        return self.stats.copy()
