import 'package:flutter/material.dart';

class CasinoScreen extends StatefulWidget {
  const CasinoScreen({super.key});

  @override
  State<CasinoScreen> createState() => _CasinoScreenState();
}

class _CasinoScreenState extends State<CasinoScreen> {
  String _selectedCategory = 'all';
  String _selectedProvider = 'all';
  final TextEditingController _searchController = TextEditingController();

  final List<Map<String, dynamic>> _mockGames = [
    {
      'id': '1',
      'name': 'Book of Dead',
      'provider': 'Play\'n GO',
      'category': 'slots',
      'imageUrl': '',
      'rtp': 96.21,
      'volatility': 'high',
    },
    {
      'id': '2',
      'name': 'Starburst',
      'provider': 'NetEnt',
      'category': 'slots',
      'imageUrl': '',
      'rtp': 96.09,
      'volatility': 'low',
    },
    {
      'id': '3',
      'name': 'Blackjack VIP',
      'provider': 'Evolution',
      'category': 'blackjack',
      'imageUrl': '',
      'rtp': 99.5,
      'volatility': 'medium',
    },
    {
      'id': '4',
      'name': 'European Roulette',
      'provider': 'NetEnt',
      'category': 'roulette',
      'imageUrl': '',
      'rtp': 97.3,
      'volatility': 'medium',
    },
    {
      'id': '5',
      'name': 'Crazy Time',
      'provider': 'Evolution',
      'category': 'live',
      'imageUrl': '',
      'rtp': 95.5,
      'volatility': 'high',
    },
    {
      'id': '6',
      'name': 'Gates of Olympus',
      'provider': 'Pragmatic Play',
      'category': 'slots',
      'imageUrl': '',
      'rtp': 96.5,
      'volatility': 'high',
    },
  ];

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Казино'),
      ),
      body: Column(
        children: [
          // Search
          Padding(
            padding: const EdgeInsets.all(16),
            child: TextField(
              controller: _searchController,
              decoration: InputDecoration(
                hintText: 'Поиск игр...',
                prefixIcon: const Icon(Icons.search),
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(12),
                ),
              ),
            ),
          ),
          // Category filter
          SizedBox(
            height: 40,
            child: ListView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              children: [
                _buildCategoryChip('Все', 'all'),
                _buildCategoryChip('Слоты', 'slots'),
                _buildCategoryChip('Live', 'live'),
                _buildCategoryChip('Блэкджек', 'blackjack'),
                _buildCategoryChip('Рулетка', 'roulette'),
              ],
            ),
          ),
          const Divider(),
          // Provider filter
          SizedBox(
            height: 40,
            child: ListView(
              scrollDirection: Axis.horizontal,
              padding: const EdgeInsets.symmetric(horizontal: 16),
              children: [
                _buildProviderChip('Все', 'all'),
                _buildProviderChip('Evolution', 'evolution'),
                _buildProviderChip('NetEnt', 'netent'),
                _buildProviderChip('Pragmatic', 'pragmatic'),
              ],
            ),
          ),
          const Divider(),
          // Games grid
          Expanded(
            child: GridView.builder(
              padding: const EdgeInsets.all(16),
              gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
                crossAxisCount: 2,
                childAspectRatio: 0.75,
                crossAxisSpacing: 12,
                mainAxisSpacing: 12,
              ),
              itemCount: _mockGames.length,
              itemBuilder: (context, index) {
                final game = _mockGames[index];
                return _buildGameCard(game);
              },
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildCategoryChip(String name, String id) {
    final isSelected = _selectedCategory == id;
    return Padding(
      padding: const EdgeInsets.only(right: 8),
      child: FilterChip(
        label: Text(name),
        selected: isSelected,
        onSelected: (selected) {
          setState(() => _selectedCategory = id);
        },
      ),
    );
  }

  Widget _buildProviderChip(String name, String id) {
    final isSelected = _selectedProvider == id;
    return Padding(
      padding: const EdgeInsets.only(right: 8),
      child: FilterChip(
        label: Text(name),
        selected: isSelected,
        onSelected: (selected) {
          setState(() => _selectedProvider = id);
        },
      ),
    );
  }

  Widget _buildGameCard(Map<String, dynamic> game) {
    return Card(
      clipBehavior: Clip.antiAlias,
      child: Stack(
        fit: StackFit.expand,
        children: [
          // Placeholder for game image
          Container(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topLeft,
                end: Alignment.bottomRight,
                colors: [
                  Colors.primaries[game['id'].hashCode % Colors.primaries.length],
                  Colors.primaries[(game['id'].hashCode + 3) % Colors.primaries.length],
                ],
              ),
            ),
            child: Icon(
              Icons.casino,
              size: 48,
              color: Colors.white.withOpacity(0.5),
            ),
          ),
          // Overlay
          Container(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topCenter,
                end: Alignment.bottomCenter,
                colors: [
                  Colors.transparent,
                  Colors.black.withOpacity(0.8),
                ],
              ),
            ),
          ),
          // Content
          Positioned(
            bottom: 0,
            left: 0,
            right: 0,
            child: Padding(
              padding: const EdgeInsets.all(8),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    game['name'],
                    style: const TextStyle(
                      color: Colors.white,
                      fontWeight: FontWeight.bold,
                      fontSize: 14,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                  const SizedBox(height: 4),
                  Text(
                    game['provider'],
                    style: TextStyle(
                      color: Colors.white.withOpacity(0.7),
                      fontSize: 12,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Row(
                    children: [
                      Expanded(
                        child: ElevatedButton(
                          onPressed: () {
                            // Launch game
                            ScaffoldMessenger.of(context).showSnackBar(
                              SnackBar(content: Text('Запуск: ${game['name']}')),
                            );
                          },
                          style: ElevatedButton.styleFrom(
                            padding: const EdgeInsets.symmetric(vertical: 4),
                          ),
                          child: const Text('Играть', style: TextStyle(fontSize: 12)),
                        ),
                      ),
                      const SizedBox(width: 4),
                      OutlinedButton(
                        onPressed: () {
                          // Demo mode
                        },
                        style: OutlinedButton.styleFrom(
                          padding: const EdgeInsets.symmetric(vertical: 4),
                          side: const BorderSide(color: Colors.white),
                        ),
                        child: const Text('Демо',
                            style: TextStyle(fontSize: 12, color: Colors.white)),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }
}
