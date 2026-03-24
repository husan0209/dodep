import 'package:flutter/material.dart';

/// Bonuses page placeholder
class BonusesPage extends StatelessWidget {
  const BonusesPage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Bonuses'),
      ),
      body: const Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.card_giftcard, size: 64),
            SizedBox(height: 16),
            Text('Bonuses - В разработке'),
          ],
        ),
      ),
    );
  }
}
