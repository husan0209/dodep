import 'package:flutter/material.dart';

/// Casino page placeholder
class CasinoPage extends StatelessWidget {
  const CasinoPage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Casino'),
      ),
      body: const Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(Icons.casino, size: 64),
            SizedBox(height: 16),
            Text('Casino - В разработке'),
          ],
        ),
      ),
    );
  }
}
