// Seed app_events for qLLM harness
db = db.getSiblingDB('events');
db.app_events.drop();
db.app_events.insertMany([
  { _id: 'e1', customerId: 'c1', type: 'login', ts: ISODate('2024-06-01T10:00:00Z') },
  { _id: 'e2', customerId: 'c1', type: 'purchase', ts: ISODate('2024-06-02T11:00:00Z') },
  { _id: 'e3', customerId: 'c2', type: 'login', ts: ISODate('2024-06-03T12:00:00Z') },
  { _id: 'e4', customerId: 'c3', type: 'signup', ts: ISODate('2024-06-04T13:00:00Z') }
]);
