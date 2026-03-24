import 'package:flutter/material.dart';

/// Wallet page placeholder
class WalletPage extends StatelessWidget {
  const WalletPage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Wallet'),
      ),
      body: const Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.account_balance_wallet, size: 64),
            SizedBox(height: 16),
            Text('Wallet - В разработке'),
          ],
        ),
      ),
    );
  }
}
