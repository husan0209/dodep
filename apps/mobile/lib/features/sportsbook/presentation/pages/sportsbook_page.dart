import 'package:flutter/material.dart';

/// Sportsbook page placeholder
class SportsbookPage extends StatelessWidget {
  const SportsbookPage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Sportsbook'),
      ),
      body: const Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.sports_soccer, size: 64),
            SizedBox(height: 16),
            Text('Sportsbook - В разработке'),
          ],
        ),
      ),
    );
  }
}
