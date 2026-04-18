CREATE TABLE IF NOT EXISTS orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_code INTEGER UNIQUE NOT NULL,
    user_id INTEGER NOT NULL,           
    total_amount INTEGER NOT NULL,         
    status TEXT NOT NULL DEFAULT 'PENDING', 
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS order_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    order_id INTEGER NOT NULL,            
    item_code TEXT NOT NULL,               
    size TEXT NOT NULL,                  
    quantity INTEGER NOT NULL,           
    price INTEGER NOT NULL,            
    FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE
);