"""
ClickHouse Data Repository for Fraud Detection
Provides data access for ML models
"""

import logging
from typing import Dict, List, Optional, Any
from datetime import datetime, timedelta

import clickhouse_connect
import pandas as pd

logger = logging.getLogger(__name__)


class ClickHouseRepository:
    """
    Repository for accessing fraud detection data from ClickHouse
    """
    
    def __init__(self, client: clickhouse_connect.Client):
        self.client = client
        
    def get_user_bets(
        self,
        user_id: int,
        hours: int = 24,
        limit: int = 1000
    ) -> pd.DataFrame:
        """Get user's betting history"""
        query = """
            SELECT 
                id,
                user_id,
                stake,
                odds,
                status,
                placed_at,
                sport_id,
                event_id,
                actual_win
            FROM bet_events
            WHERE user_id = %(user_id)s
              AND event_time >= now() - INTERVAL %(hours)s HOUR
            ORDER BY event_time DESC
            LIMIT %(limit)s
        """
        
        result = self.client.query(
            query,
            {"user_id": user_id, "hours": hours, "limit": limit}
        )
        
        if not result.result_rows:
            return pd.DataFrame()
            
        columns = [col.name for col in result.column_names]
        return pd.DataFrame(result.result_rows, columns=columns)
    
    def get_user_transactions(
        self,
        user_id: int,
        hours: int = 24,
        limit: int = 1000
    ) -> pd.DataFrame:
        """Get user's transaction history"""
        query = """
            SELECT 
                id,
                user_id,
                type,
                amount,
                status,
                payment_method,
                created_at
            FROM financial_reports
            WHERE user_id = %(user_id)s
              AND created_at >= now() - INTERVAL %(hours)s HOUR
            ORDER BY created_at DESC
            LIMIT %(limit)s
        """
        
        result = self.client.query(
            query,
            {"user_id": user_id, "hours": hours, "limit": limit}
        )
        
        if not result.result_rows:
            return pd.DataFrame()
            
        columns = [col.name for col in result.column_names]
        return pd.DataFrame(result.result_rows, columns=columns)
    
    def get_user_login_history(
        self,
        user_id: int,
        hours: int = 168,  # 7 days
        limit: int = 100
    ) -> pd.DataFrame:
        """Get user's login history"""
        query = """
            SELECT 
                user_id,
                ip,
                country,
                device_fingerprint,
                event_time,
                success
            FROM user_events
            WHERE user_id = %(user_id)s
              AND event_type = 'login'
              AND event_time >= now() - INTERVAL %(hours)s HOUR
            ORDER BY event_time DESC
            LIMIT %(limit)s
        """
        
        result = self.client.query(
            query,
            {"user_id": user_id, "hours": hours, "limit": limit}
        )
        
        if not result.result_rows:
            return pd.DataFrame()
            
        columns = [col.name for col in result.column_names]
        return pd.DataFrame(result.result_rows, columns=columns)
    
    def get_user_bonus_history(
        self,
        user_id: int,
        days: int = 30,
        limit: int = 100
    ) -> pd.DataFrame:
        """Get user's bonus history"""
        query = """
            SELECT 
                id,
                user_id,
                bonus_type,
                amount,
                status,
                wagering_requirement,
                wagering_completed,
                created_at
            FROM fraud_signals
            WHERE user_id = %(user_id)s
              AND event_time >= now() - INTERVAL %(days)s DAY
            ORDER BY event_time DESC
            LIMIT %(limit)s
        """
        
        result = self.client.query(
            query,
            {"user_id": user_id, "days": days, "limit": limit}
        )
        
        if not result.result_rows:
            return pd.DataFrame()
            
        columns = [col.name for col in result.column_names]
        return pd.DataFrame(result.result_rows, columns=columns)
    
    def get_user_profile(self, user_id: int) -> Optional[Dict[str, Any]]:
        """Get user profile data"""
        query = """
            SELECT 
                user_id,
                country,
                currency,
                created_at,
                kyc_level,
                status
            FROM user_events
            WHERE user_id = %(user_id)s
            LIMIT 1
        """
        
        result = self.client.query(query, {"user_id": user_id})
        
        if not result.result_rows:
            return None
            
        return dict(zip(result.column_names, result.result_rows[0]))
    
    def get_fraud_statistics(self, hours: int = 24) -> Dict[str, Any]:
        """Get fraud detection statistics"""
        query = """
            SELECT 
                count() as total_events,
                sum(is_fraud) as fraud_count,
                avg(risk_score) as avg_risk_score
            FROM fraud_signals
            WHERE event_time >= now() - INTERVAL %(hours)s HOUR
        """
        
        result = self.client.query(query, {"hours": hours})
        
        if not result.result_rows:
            return {}
            
        row = result.result_rows[0]
        return {
            "total_events": row[0],
            "fraud_count": row[1],
            "avg_risk_score": row[2],
        }
    
    def insert_fraud_signal(self, data: Dict[str, Any]):
        """Insert fraud detection result"""
        query = """
            INSERT INTO fraud_signals (
                event_time,
                user_id,
                fraud_type,
                risk_score,
                is_fraud,
                features,
                explanation
            ) VALUES (
                %(event_time)s,
                %(user_id)s,
                %(fraud_type)s,
                %(risk_score)s,
                %(is_fraud)s,
                %(features)s,
                %(explanation)s
            )
        """
        
        self.client.command(query, data)
        
    def get_aggregated_user_features(self, user_id: int) -> Dict[str, Any]:
        """Get aggregated features for a user"""
        features = {}
        
        # Get betting stats
        bets_query = """
            SELECT 
                count() as total_bets,
                sum(stake) as total_staked,
                avg(stake) as avg_stake,
                max(stake) as max_stake,
                sum(actual_win) as total_won,
                countIf(status = 'won') as wins,
                countIf(status = 'lost') as losses
            FROM bet_events
            WHERE user_id = %(user_id)s
              AND event_time >= now() - INTERVAL 30 DAY
        """
        
        result = self.client.query(bets_query, {"user_id": user_id})
        if result.result_rows:
            row = result.result_rows[0]
            features.update({
                "total_bets_30d": row[0],
                "total_staked_30d": float(row[1]) if row[1] else 0,
                "avg_stake": float(row[2]) if row[2] else 0,
                "max_stake": float(row[3]) if row[3] else 0,
                "total_won_30d": float(row[4]) if row[4] else 0,
                "wins_30d": row[5],
                "losses_30d": row[6],
            })
            
        # Get transaction stats
        tx_query = """
            SELECT 
                count() as total_txs,
                sum(amount) as total_amount,
                countIf(status = 'failed') as failed_txs
            FROM financial_reports
            WHERE user_id = %(user_id)s
              AND created_at >= now() - INTERVAL 30 DAY
        """
        
        result = self.client.query(tx_query, {"user_id": user_id})
        if result.result_rows:
            row = result.result_rows[0]
            features.update({
                "total_transactions_30d": row[0],
                "total_transaction_amount_30d": float(row[1]) if row[1] else 0,
                "failed_transactions_30d": row[2],
            })
            
        return features
