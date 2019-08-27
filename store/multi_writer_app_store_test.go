package store

import (
	"bytes"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/loomnetwork/go-loom/config"
	"github.com/loomnetwork/go-loom/util"
	"github.com/loomnetwork/loomchain/db"
	"github.com/loomnetwork/loomchain/log"
	"github.com/stretchr/testify/suite"
)

type MultiWriterAppStoreTestSuite struct {
	suite.Suite
}

func (m *MultiWriterAppStoreTestSuite) SetupTest() {
	log.Setup("debug", "file://-")
}

func TestMultiWriterAppStoreTestSuite(t *testing.T) {
	suite.Run(t, new(MultiWriterAppStoreTestSuite))
}

func (m *MultiWriterAppStoreTestSuite) TestEnableDisableMultiWriterAppStore() {
	require := m.Require()
	store, _, _, err := mockMultiWriterStore(10)
	require.NoError(err)

	// vm keys should be written to both the IAVL & EVM store
	store.Set(evmDBFeatureKey, []byte{})
	store.Set(vmPrefixKey("abcd"), []byte("hello"))
	store.Set(vmPrefixKey("abcde"), []byte("world"))
	store.Set(vmPrefixKey("evmStore"), []byte("yes"))
	store.Set(vmPrefixKey("aaaa"), []byte("yes"))
	store.Set([]byte("abcd"), []byte("NewData"))

	rangeData := store.Range(vmPrefix)
	require.Equal(4, len(rangeData))
	require.True(store.Has([]byte("abcd")))

	// vm keys should now only be written to the EVM store
	store.Set(evmDBFeatureKey, []byte{1})
	store.Set(vmPrefixKey("gg"), []byte("world"))
	store.Set(vmPrefixKey("dd"), []byte("yes"))
	store.Set(vmPrefixKey("vv"), []byte("yes"))
	store.Set([]byte("dcba"), []byte("MoreData"))

	rangeData = store.Range(vmPrefix)
	require.Equal(7, len(rangeData))
	require.True(store.Has([]byte("abcd")))
	require.True(store.Has([]byte("dcba")))
}

func (m *MultiWriterAppStoreTestSuite) TestMultiWriterAppStoreDelete() {
	require := m.Require()
	store, _, _, err := mockMultiWriterStore(10)
	require.NoError(err)

	// vm keys should be written to both the IAVL & EVM store
	store.Set(evmDBFeatureKey, []byte{})
	store.Set(vmPrefixKey("abcd"), []byte("hello"))
	store.Set(vmPrefixKey("abcde"), []byte("world"))
	store.Set(vmPrefixKey("evmStore"), []byte("yes"))
	store.Set(vmPrefixKey("aaaa"), []byte("yes"))
	store.Set([]byte("vmroot"), []byte("SSSSSSSSSSSSS"))
	store.Set([]byte("abcd"), []byte("NewData"))

	store.Delete(vmPrefixKey("abcd"))
	require.False(store.Has(vmPrefixKey("abcd")))

	rangeData := store.Range(vmPrefix)
	require.Equal(3, len(rangeData))
	require.True(store.Has([]byte("vmroot")))
	require.True(store.Has([]byte("abcd")))

	// vm keys should be written to the EVM store
	store.Set(evmDBFeatureKey, []byte{1})
	rangeData = store.Range(vmPrefix)
	require.Equal(3, len(rangeData))
	require.Equal([]byte("SSSSSSSSSSSSS"), store.Get([]byte("vmroot")))

	store.Set(vmPrefixKey("gg"), []byte("world"))
	store.Set(vmPrefixKey("dd"), []byte("yes"))
	store.Set(vmPrefixKey("vv"), []byte("yes"))
	store.Delete(vmPrefixKey("vv"))
	require.False(store.Has(vmPrefixKey("vv")))

	rangeData = store.Range(vmPrefix)
	require.Equal(5, len(rangeData))
	require.True(store.Has([]byte("abcd")))
}

func (m *MultiWriterAppStoreTestSuite) TestMultiWriterAppStoreSnapShot() {
	require := m.Require()
	store, _, _, err := mockMultiWriterStore(10)
	require.NoError(err)

	store.Set(evmDBFeatureKey, []byte{1})
	store.Set(vmPrefixKey("abcd"), []byte("hello"))
	store.Set(vmPrefixKey("abcde"), []byte("world"))
	store.Set(vmPrefixKey("evmStore"), []byte("yes"))
	store.Set(vmPrefixKey("aaaa"), []byte("yes"))
	store.Set([]byte("ssssvvv"), []byte("SSSSSSSSSSSSS"))
	store.Set([]byte("abcd"), []byte("NewData"))
	_, _, err = store.SaveVersion()
	require.NoError(err)

	store.Set(vmPrefixKey("abcd"), []byte("hellooooooo"))
	store.Set(vmPrefixKey("abcde"), []byte("vvvvvvvvv"))
	store.Set([]byte("abcd"), []byte("asdfasdf"))

	snapshot := store.GetSnapshot()
	require.Equal([]byte("hello"), snapshot.Get(vmPrefixKey("abcd")))
	require.Equal([]byte("NewData"), snapshot.Get([]byte("abcd")))
	require.Equal([]byte("world"), snapshot.Get(vmPrefixKey("abcde")))

	_, _, err = store.SaveVersion()
	require.NoError(err)

	snapshot = store.GetSnapshot()
	require.Equal([]byte("asdfasdf"), snapshot.Get([]byte("abcd")))
	require.Equal([]byte("hellooooooo"), snapshot.Get(vmPrefixKey("abcd")))
	require.Equal([]byte("vvvvvvvvv"), snapshot.Get(vmPrefixKey("abcde")))
}

func (m *MultiWriterAppStoreTestSuite) TestMultiWriterAppStoreSnapShotFlushInterval() {
	require := m.Require()
	// flush data to disk every 2 blocks
	store, _, _, err := mockMultiWriterStore(2)
	require.NoError(err)

	// the first version go to memory
	store.Set([]byte("test1"), []byte("test1"))
	store.Set([]byte("test2"), []byte("test2"))
	_, version, err := store.SaveVersion()
	require.NoError(err)
	require.Equal(int64(1), version)

	store.Set([]byte("test1"), []byte("test1v2"))
	store.Set([]byte("test2"), []byte("test2v2"))

	// this snapshot is from memory
	snapshotv1 := store.GetSnapshot()
	require.Equal([]byte("test1"), snapshotv1.Get([]byte("test1")))
	require.Equal([]byte("test2"), snapshotv1.Get([]byte("test2")))

	// this flushes all data to disk
	_, version, err = store.SaveVersion()
	require.NoError(err)

	// get snapshotv2
	snapshotv2 := store.GetSnapshot()
	require.Equal([]byte("test1v2"), snapshotv2.Get([]byte("test1")))
	require.Equal([]byte("test2v2"), snapshotv2.Get([]byte("test2")))

	// this snapshotv1 should still be accessible
	require.Equal([]byte("test1"), snapshotv1.Get([]byte("test1")))
	require.Equal([]byte("test2"), snapshotv1.Get([]byte("test2")))
}

func (m *MultiWriterAppStoreTestSuite) TestMultiWriterAppStoreSnapShotRange() {
	require := m.Require()
	store, _, _, err := mockMultiWriterStore(10)
	require.NoError(err)

	store.Set(evmDBFeatureKey, []byte{1})
	store.Set(vmPrefixKey("abcd"), []byte("hello"))
	store.Set(vmPrefixKey("abcde"), []byte("world"))
	store.Set(vmPrefixKey("evmStore"), []byte("yes"))
	store.Set(vmPrefixKey("aaaa"), []byte("yes"))
	store.Set([]byte("ssssvvv"), []byte("SSSSSSSSSSSSS"))
	store.Set([]byte("abcd"), []byte("NewData"))
	store.Set([]byte("uuuu"), []byte("SSSSSSSSSSSSS"))
	store.Set([]byte("sssss"), []byte("NewData"))

	snapshot := store.GetSnapshot()
	rangeData := snapshot.Range(vmPrefix)
	require.Equal(0, len(rangeData))
	_, _, err = store.SaveVersion()
	require.NoError(err)

	snapshot = store.GetSnapshot()
	rangeData = snapshot.Range(vmPrefix)
	require.Equal(4+1, len(rangeData)) // +1 for evm root stored by EVM store
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("abcd")), []byte("hello")))
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("abcde")), []byte("world")))
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("evmStore")), []byte("yes")))
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("aaaa")), []byte("yes")))

	// Modifications shouldn't be visible in the snapshot until the next SaveVersion()
	store.Delete(vmPrefixKey("abcd"))
	store.Delete([]byte("ssssvvv"))

	snapshot = store.GetSnapshot()
	rangeData = snapshot.Range(vmPrefix)
	require.Equal(4+1, len(rangeData)) // +1 for evm root stored by EVM store
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("abcd")), []byte("hello")))
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("abcde")), []byte("world")))
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("evmStore")), []byte("yes")))
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("aaaa")), []byte("yes")))

	_, _, err = store.SaveVersion()
	require.NoError(err)

	snapshot = store.GetSnapshot()
	rangeData = snapshot.Range(vmPrefix)
	require.Equal(3+1, len(rangeData))                       // +1 for evm root stored by EVM store
	require.Equal(0, len(snapshot.Get(vmPrefixKey("abcd")))) // has been deleted
	require.Equal(0, len(snapshot.Get([]byte("ssssvvv"))))   // has been deleted
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("abcde")), []byte("world")))
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("evmStore")), []byte("yes")))
	require.Equal(0, bytes.Compare(snapshot.Get(vmPrefixKey("aaaa")), []byte("yes")))
}

func (m *MultiWriterAppStoreTestSuite) TestMultiWriterAppStoreSaveVersion() {
	require := m.Require()
	store, _, _, err := mockMultiWriterStore(10)
	require.NoError(err)

	// vm keys should be written to the EVM store
	store.Set(evmDBFeatureKey, []byte{1})
	store.Set(vmPrefixKey("abcd"), []byte("hello"))
	store.Set(vmPrefixKey("abcde"), []byte("world"))
	store.Set(vmPrefixKey("evmStore"), []byte("yes"))
	store.Set(vmPrefixKey("aaaa"), []byte("yes"))
	store.Set([]byte("abcd"), []byte("NewData"))
	store.Set([]byte("evmStore"), []byte("iavlStore"))
	store.Set(vmPrefixKey("gg"), []byte("world"))
	store.Set(vmPrefixKey("dd"), []byte("yes"))
	store.Set(vmPrefixKey("vv"), []byte("yes"))

	_, version, err := store.SaveVersion()
	require.Equal(int64(1), version)
	require.NoError(err)

	require.Equal([]byte("hello"), store.Get(vmPrefixKey("abcd")))
	require.Equal([]byte("NewData"), store.Get([]byte("abcd")))
	require.True(store.Has(vmPrefixKey("gg")))
	store.Delete(vmPrefixKey("gg"))

	dataRange := store.Range(vmPrefix)
	require.Equal(6+1, len(dataRange)) // +1 is for the evm root that written by the EVM store itself

	_, version, err = store.SaveVersion()
	require.Equal(int64(2), version)
	require.NoError(err)

	require.Equal([]byte("hello"), store.Get(vmPrefixKey("abcd")))
	require.Equal([]byte("NewData"), store.Get([]byte("abcd")))
	require.False(store.Has(vmPrefixKey("gg")))
}

func (m *MultiWriterAppStoreTestSuite) TestPruningEvmKeys() {
	require := m.Require()
	store, iavlStore, evmStore, err := mockMultiWriterStore(10)
	require.NoError(err)

	// write some vm keys to iavl store
	iavlStore.Set(vmPrefixKey("abcde"), []byte("world"))
	iavlStore.Set(vmPrefixKey("aaaa"), []byte("yes"))
	iavlStore.Set(vmPrefixKey("abcd"), []byte("NewData"))
	iavlStore.Set(vmPrefixKey("evmStore"), []byte("iavlStore"))
	iavlStore.Set(vmPrefixKey("gg"), []byte("world"))
	iavlStore.Set(vmPrefixKey("dd"), []byte("yes"))
	iavlStore.Set(vmPrefixKey("vv"), []byte("yes"))
	_, version, err := store.SaveVersion()
	require.Equal(int64(1), version)
	require.NoError(err)

	newStore, err := NewMultiWriterAppStore(iavlStore, evmStore, false)
	require.NoError(err)

	rangeData := iavlStore.Range([]byte("vm"))
	require.Equal(7, len(rangeData))

	// prune 2 vm keys per block
	cfg := config.DefaultConfig()
	cfg.AppStore.NumEvmKeysToPrune = 5
	configBytes, err := proto.Marshal(cfg)
	require.NoError(err)
	newStore.Set([]byte(configKey), configBytes)
	// enable pruning with appstore v3.2
	newStore.Set(appStoreVersion3_2, []byte{1})
	// prune VM keys
	_, version, err = newStore.SaveVersion()
	require.Equal(int64(2), version)
	require.NoError(err)

	// expect number of vm keys to be 7-3 = 4
	// the number of deleted keys per will not match the number of NumEvmKeysToPrune
	// due to iavlstore.RangeWithLimit please read the explanation from TestIAVLRangeWithlimit
	rangeData = iavlStore.Range([]byte("vm"))
	require.Equal(4, len(rangeData))

	// prune VM keys
	_, version, err = newStore.SaveVersion()
	require.Equal(int64(3), version)
	require.NoError(err)

	// expect number of vm keys to be 4-3 = 1
	rangeData = iavlStore.Range([]byte("vm"))
	require.Equal(1, len(rangeData))

	// prune VM keys
	_, version, err = newStore.SaveVersion()
	require.Equal(int64(4), version)
	require.NoError(err)

	// expect number of vm keys to be = 0
	rangeData = iavlStore.Range([]byte("vm"))
	require.Equal(0, len(rangeData))
}

func (m *MultiWriterAppStoreTestSuite) TestIAVLRangeWithlimit() {
	require := m.Require()
	store, iavlStore, _, err := mockMultiWriterStore(10)
	require.NoError(err)

	// write some vm keys to iavl store
	iavlStore.Set(vmPrefixKey("abcde"), []byte("world"))
	iavlStore.Set(vmPrefixKey("aaaa"), []byte("yes"))
	iavlStore.Set(vmPrefixKey("abcd"), []byte("NewData"))
	iavlStore.Set(vmPrefixKey("evmStore"), []byte("iavlStore"))
	iavlStore.Set(vmPrefixKey("gg"), []byte("world"))
	iavlStore.Set(vmPrefixKey("dd"), []byte("yes"))
	iavlStore.Set(vmPrefixKey("vv"), []byte("yes"))
	_, _, err = store.SaveVersion()
	require.NoError(err)

	// Note: this is an unpredictable behavior due to our prefix system
	// the number of returned keys might be less than the limit even though the limit is less than
	// the total number of target-prefixed keys in the store because iavl store excludes non-prefixed keys
	// In this case, vmroot is excluded from list so the expect number is 4
	rangeData := iavlStore.RangeWithLimit([]byte("vm"), 5)
	require.Equal(4, len(rangeData))
}

func mockMultiWriterStore(flushInterval int64) (*MultiWriterAppStore, *IAVLStore, *EvmStore, error) {
	memDb, _ := db.LoadMemDB()
	iavlStore, err := NewIAVLStore(memDb, 0, 0, flushInterval)
	if err != nil {
		return nil, nil, nil, err
	}
	memDb, _ = db.LoadMemDB()
	evmStore := NewEvmStore(memDb, 100)
	multiWriterStore, err := NewMultiWriterAppStore(iavlStore, evmStore, false)
	if err != nil {
		return nil, nil, nil, err
	}
	return multiWriterStore, iavlStore, evmStore, nil
}

func vmPrefixKey(key string) []byte {
	return util.PrefixKey([]byte("vm"), []byte(key))
}
