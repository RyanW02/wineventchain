use std::{marker::PhantomData, sync::Arc};

use crate::{AgentError, Result};
use parking_lot::Mutex;
use serde::{de::DeserializeOwned, Serialize};
use std::str;

pub struct DiskQueue<T>
where
    T: Clone + DeserializeOwned + Serialize,
{
    db: Arc<sled::Db>,
    tree: Mutex<sled::Tree>,
    options: DiskQueueOptions,
    _marker: PhantomData<T>,
}

pub struct DiskQueueOptions {
    pub tree_name: String,
    pub max_size: Option<usize>,
}

impl<T> DiskQueue<T>
where
    T: Clone + DeserializeOwned + Serialize,
{
    pub fn new(db: Arc<sled::Db>, options: DiskQueueOptions) -> Result<Self> {
        let tree = db.open_tree(options.tree_name.as_str())?;

        Ok(Self {
            db,
            tree: Mutex::new(tree),
            options,
            _marker: PhantomData,
        })
    }

    pub fn push(&self, item: T) -> Result<u64> {
        let serialized = serde_json::to_vec(&item)?;

        let tree = &*self.tree.lock();

        if let Some(max_size) = self.options.max_size {
            if tree.len() >= max_size {
                let _ = tree.pop_min()?;
            }
        }

        let key = self.db.generate_id()?;
        tree.insert(encode_binary_key(key), &serialized[..])?;

        Ok(key)
    }

    pub fn get_by_id(&self, key: u64) -> Result<Option<T>> {
        let tree = &*self.tree.lock();
        let value = tree.get(encode_binary_key(key))?;

        match value {
            Some(value) => {
                let value = value.as_ref();
                let item = serde_json::from_slice(value)?;
                Ok(Some(item))
            }
            None => Ok(None),
        }
    }

    pub fn remove(&self, key: u64) -> Result<()> {
        let tree = &*self.tree.lock();
        tree.remove(encode_binary_key(key))?;
        Ok(())
    }

    pub fn update(&self, key: u64, item: T) -> Result<()> {
        let serialized = serde_json::to_vec(&item)?;
        let tree = &*self.tree.lock();
        tree.insert(encode_binary_key(key), &serialized[..])?;
        Ok(())
    }

    pub fn iter(&self) -> DiskQueueIterator<'_, T> {
        self.into_iter()
    }
}

pub struct DiskQueueIterator<'a, T>
where
    T: Clone + DeserializeOwned + Serialize,
{
    queue: &'a DiskQueue<T>,
    current: Option<u64>,
}

impl<'a, T> IntoIterator for &'a DiskQueue<T>
where
    T: Clone + DeserializeOwned + Serialize,
{
    type Item = Result<(u64, T)>;
    type IntoIter = DiskQueueIterator<'a, T>;

    fn into_iter(self) -> Self::IntoIter {
        DiskQueueIterator {
            queue: self,
            current: None,
        }
    }
}

impl<'a, T> Iterator for DiskQueueIterator<'a, T>
where
    T: Clone + DeserializeOwned + Serialize,
{
    type Item = Result<(u64, T)>;

    fn next(&mut self) -> Option<Self::Item> {
        let tree = self.queue.tree.lock();

        // Find next highest value after current key
        // Iterator::find is equivalent to calling filter(..).next()
        let (key, next_raw) = match tree
            .iter()
            .map(|r| {
                r.map(|(k, v)| (decode_binary_key(&k[..]), v))
                    .map_err(AgentError::from)
                    .and_then(|(r, v)| r.map(|k| (k, v)))
            })
            .find(|r| match r {
                Ok((k, _)) => match self.current {
                    Some(current) => *k > current,
                    None => true,
                },
                Err(_) => true, // Keep iterating so that we can return the error at the end
            })? {
            Ok(v) => v,
            Err(e) => return Some(Err(e)),
        };

        // Free lock before doing deserialization
        drop(tree);

        let value = match serde_json::from_slice(&next_raw) {
            Ok(value) => value,
            Err(e) => return Some(Err(AgentError::JsonError(e))),
        };

        self.current = Some(key);
        Some(Ok((key, value)))
    }
}

fn encode_binary_key(key: u64) -> String {
    format!("{:064b}", key)
}

fn decode_binary_key(key: &[u8]) -> Result<u64> {
    u64::from_str_radix(str::from_utf8(key)?, 2).map_err(AgentError::from)
}

#[cfg(test)]
mod tests {
    use super::*;
    use rand::{distributions::Alphanumeric, Rng};
    use sled::Batch;
    use tempdir::TempDir;

    #[test]
    fn test_tree_order() -> Result<()> {
        let tmp = TempDir::new("sled").expect("tempdir failed");
        let db = sled::open(tmp.path().join("test.db")).expect("sled::open failed");
        let tree = db.open_tree("tree").expect("open_tree failed");

        // Generate some spaced out IDs
        let mut ids = Vec::with_capacity(3334);
        (0u64..1_000).for_each(|id| ids.push(id));
        (1_000u64..5_000).step_by(5).for_each(|id| ids.push(id));
        (5_000u64..10_000).step_by(10).for_each(|id| ids.push(id));
        (10_000u64..100_000).step_by(87).for_each(|id| ids.push(id));

        let mut batch = Batch::default();
        for id in &ids {
            batch.insert(encode_binary_key(*id).as_str(), b"some_value");
        }

        tree.apply_batch(batch).expect("failed to apply batch");

        // Test first and last values
        let (first_key, first_value) = tree.first()?.expect("no first");
        let (last_key, last_value) = tree.last()?.expect("no last");

        assert_eq!(decode_binary_key(&first_key[..])?, 0);
        assert_eq!(first_value.as_ref(), b"some_value");
        assert_eq!(decode_binary_key(&last_key[..])?, 99958);
        assert_eq!(last_value.as_ref(), b"some_value");

        // Iterate through all keys
        let iter = tree
            .iter()
            .map(|r| r.unwrap())
            .zip(tree.iter().map(|r| r.unwrap()).skip(1));
        for ((cur_key, cur_value), (next_key, next_value)) in iter {
            let cur_id = decode_binary_key(&cur_key[..])?;
            let next_id = decode_binary_key(&next_key[..])?;
            assert!(cur_id < next_id, "cur_id: {}, next_id: {}", cur_id, next_id);

            assert_eq!(cur_value.as_ref(), b"some_value");
            assert_eq!(next_value.as_ref(), b"some_value");
        }

        drop(db);
        Ok(())
    }

    #[test]
    fn test_insert() {
        let queue = build_queue(Some(100));

        let key = queue.push("one".to_string()).expect("push failed");
        let item = queue
            .get_by_id(key)
            .expect("get_by_id failed")
            .expect("no item");

        assert_eq!(item, "one");
    }

    #[test]
    fn test_update() {
        let queue = build_queue(Some(100));

        let key = queue.push("one".to_string()).expect("push failed");
        queue.update(key, "two".to_string()).expect("update failed");
        let item = queue
            .get_by_id(key)
            .expect("get_by_id failed")
            .expect("no item");

        assert_eq!(item, "two");
    }

    #[test]
    fn test_remove() {
        let queue = build_queue(Some(100));

        let key = queue.push("one".to_string()).expect("push failed");
        queue.remove(key).expect("remove failed");
        let item = queue.get_by_id(key).expect("get_by_id failed");

        assert_eq!(item, None);
    }

    #[test]
    fn test_iter() {
        let queue = build_queue(Some(100));

        let keys = (0..100)
            .map(|_| queue.push(random_string(10)).expect("push failed"))
            .collect::<Vec<_>>();

        queue
            .iter()
            .map(|r| r.expect("iter failed"))
            .zip(keys.iter())
            .for_each(|((k, v), expected_k)| {
                assert_eq!(k, *expected_k);
                assert_eq!(v.len(), 10);
            });
    }

    #[test]
    fn test_iter_with_remove() {
        let queue = build_queue(Some(100));

        let mut keys = (0..100)
            .map(|_| queue.push(random_string(10)).expect("push failed"))
            .collect::<Vec<_>>();

        // Remove every other key
        (0..100)
            .step_by(2)
            .for_each(|i| queue.remove(keys[i]).expect("remove failed"));
        keys.retain(|k| k % 2 == 1);

        queue
            .iter()
            .map(|r| r.expect("iter failed"))
            .zip(keys.iter())
            .for_each(|((k, v), expected_k)| {
                assert_eq!(k, *expected_k);
                assert_eq!(v.len(), 10);
            });
    }

    #[test]
    fn test_iter_with_update() {
        let queue = build_queue(Some(100));

        let keys = (0..100)
            .map(|_| queue.push(random_string(10)).expect("push failed"))
            .collect::<Vec<_>>();

        // Update every other key
        (0..100).step_by(2).for_each(|i| {
            queue
                .update(keys[i], random_string(15))
                .expect("update failed")
        });

        queue
            .iter()
            .map(|r| r.expect("iter failed"))
            .zip(keys.iter())
            .for_each(|((k, v), expected_k)| {
                assert_eq!(k, *expected_k);

                if k % 2 == 0 {
                    assert_eq!(v.len(), 15);
                } else {
                    assert_eq!(v.len(), 10);
                }
            });
    }

    #[test]
    fn test_max_size() -> Result<()> {
        let queue = build_queue(Some(50));

        let keys = (0..100)
            .map(|_| queue.push(random_string(10)).expect("push failed"))
            .collect::<Vec<_>>();

        let in_tree = queue.iter().collect::<Result<Vec<_>>>()?;
        assert_eq!(in_tree.len(), 50);

        in_tree.iter().enumerate().for_each(|(i, (k, _))| {
            assert_eq!(*k, keys[i + 50]);
        });

        Ok(())
    }

    fn build_queue<T>(max_size: Option<usize>) -> DiskQueue<T>
    where
        T: Clone + DeserializeOwned + Serialize,
    {
        let tmp = TempDir::new("sled").expect("tempdir failed");
        let db = sled::open(tmp.path().join("test.db")).expect("sled::open failed");
        DiskQueue::<T>::new(
            Arc::new(db),
            DiskQueueOptions {
                tree_name: "test".to_string(),
                max_size,
            },
        )
        .expect("DiskQueue::new failed")
    }

    fn random_string(length: usize) -> String {
        rand::thread_rng()
            .sample_iter(&Alphanumeric)
            .take(length)
            .map(char::from)
            .collect()
    }
}
