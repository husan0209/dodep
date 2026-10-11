import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

/// Profile page
class ProfilePage extends StatelessWidget {
  const ProfilePage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Profile'),
      ),
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          const Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.person, size: 64),
                SizedBox(height: 16),
                Text('Profile - В разработке'),
              ],
            ),
          ),
          const SizedBox(height: 24),
          _buildMenuItem(
            Icons.group_outlined,
            'Партнёрская программа',
            () => context.go('/affiliate'),
          ),
          _buildMenuItem(
            Icons.shield_outlined,
            'Ответственная игра',
            () => context.go('/responsible-gambling'),
          ),
        ],
      ),
    );
  }

  Widget _buildMenuItem(IconData icon, String title, VoidCallback onTap) {
    return Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: ListTile(
        leading: Icon(icon),
        title: Text(title),
        trailing: const Icon(Icons.chevron_right),
        onTap: onTap,
      ),
    );
  }
}
