package iavl

import (
	"math/rand"
	"sort"
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/iavl/fastnode"
)

func TestIterator_NewIterator_NilTree_Failure(t *testing.T) {
	start, end := []byte{'a'}, []byte{'c'}
	ascending := true

	performTest := func(t *testing.T, itr dbm.Iterator) {
		require.NotNil(t, itr)
		require.False(t, itr.Valid())
		actualsStart, actualEnd := itr.Domain()
		require.Equal(t, start, actualsStart)
		require.Equal(t, end, actualEnd)
		require.Error(t, itr.Error())
	}

	t.Run("Iterator", func(t *testing.T) {
		itr := NewIterator(start, end, ascending, nil)
		performTest(t, itr)
		require.ErrorIs(t, errIteratorNilTreeGiven, itr.Error())
	})

	t.Run("Fast Iterator", func(t *testing.T) {
		itr := NewFastIterator(start, end, ascending, nil)
		performTest(t, itr)
		require.ErrorIs(t, errFastIteratorNilNdbGiven, itr.Error())
	})

	t.Run("Unsaved Fast Iterator", func(t *testing.T) {
		itr := NewUnsavedFastIterator(start, end, ascending, nil, map[string]*fastnode.Node{}, map[string]interface{}{})
		performTest(t, itr)
		require.ErrorIs(t, errFastIteratorNilNdbGiven, itr.Error())
	})
}

func TestUnsavedFastIterator_NewIterator_NilAdditions_Failure(t *testing.T) {
	start, end := []byte{'a'}, []byte{'c'}
	ascending := true

	performTest := func(t *testing.T, itr dbm.Iterator) {
		require.NotNil(t, itr)
		require.False(t, itr.Valid())
		actualsStart, actualEnd := itr.Domain()
		require.Equal(t, start, actualsStart)
		require.Equal(t, end, actualEnd)
		require.Error(t, itr.Error())
	}

	t.Run("Nil additions given", func(t *testing.T) {
		tree, err := NewMutableTree(dbm.NewMemDB(), 0, false)
		require.NoError(t, err)
		itr := NewUnsavedFastIterator(start, end, ascending, tree.ndb, nil, tree.unsavedFastNodeRemovals)
		performTest(t, itr)
		require.ErrorIs(t, errUnsavedFastIteratorNilAdditionsGiven, itr.Error())
	})

	t.Run("Nil removals given", func(t *testing.T) {
		tree, err := NewMutableTree(dbm.NewMemDB(), 0, false)
		require.NoError(t, err)
		itr := NewUnsavedFastIterator(start, end, ascending, tree.ndb, tree.unsavedFastNodeAdditions, nil)
		performTest(t, itr)
		require.ErrorIs(t, errUnsavedFastIteratorNilRemovalsGiven, itr.Error())
	})

	t.Run("All nil", func(t *testing.T) {
		itr := NewUnsavedFastIterator(start, end, ascending, nil, nil, nil)
		performTest(t, itr)
		require.ErrorIs(t, errFastIteratorNilNdbGiven, itr.Error())
	})

	t.Run("Additions and removals are nil", func(t *testing.T) {
		tree, err := NewMutableTree(dbm.NewMemDB(), 0, false)
		require.NoError(t, err)
		itr := NewUnsavedFastIterator(start, end, ascending, tree.ndb, nil, nil)
		performTest(t, itr)
		require.ErrorIs(t, errUnsavedFastIteratorNilAdditionsGiven, itr.Error())
	})
}

func TestIterator_Empty_Invalid(t *testing.T) {
	config := &iteratorTestConfig{
		startByteToSet: 'a',
		endByteToSet:   'z',
		startIterate:   []byte("a"),
		endIterate:     []byte("a"),
		ascending:      true,
	}

	performTest := func(t *testing.T, itr dbm.Iterator, mirror [][]string) {
		require.Equal(t, 0, len(mirror))
		require.False(t, itr.Valid())
	}

	t.Run("Iterator", func(t *testing.T) {
		itr, mirror := setupIteratorAndMirror(t, config)
		performTest(t, itr, mirror)
	})

	t.Run("Fast Iterator", func(t *testing.T) {
		itr, mirror := setupFastIteratorAndMirror(t, config)
		performTest(t, itr, mirror)
	})

	t.Run("Unsaved Fast Iterator", func(t *testing.T) {
		itr, mirror := setupUnsavedFastIterator(t, config)
		performTest(t, itr, mirror)
	})
}

func TestIterator_Basic_Ranged_Ascending_Success(t *testing.T) {
	config := &iteratorTestConfig{
		startByteToSet: 'a',
		endByteToSet:   'z',
		startIterate:   []byte("e"),
		endIterate:     []byte("w"),
		ascending:      true,
	}
	iteratorSuccessTest(t, config)
}

func TestIterator_Basic_Ranged_Descending_Success(t *testing.T) {
	config := &iteratorTestConfig{
		startByteToSet: 'a',
		endByteToSet:   'z',
		startIterate:   []byte("e"),
		endIterate:     []byte("w"),
		ascending:      false,
	}
	iteratorSuccessTest(t, config)
}

func TestIterator_Basic_Full_Ascending_Success(t *testing.T) {
	config := &iteratorTestConfig{
		startByteToSet: 'a',
		endByteToSet:   'z',
		startIterate:   nil,
		endIterate:     nil,
		ascending:      true,
	}

	iteratorSuccessTest(t, config)
}

func TestIterator_Basic_Full_Descending_Success(t *testing.T) {
	config := &iteratorTestConfig{
		startByteToSet: 'a',
		endByteToSet:   'z',
		startIterate:   nil,
		endIterate:     nil,
		ascending:      false,
	}
	iteratorSuccessTest(t, config)
}

func mirrorToSlice(mirror map[string]string) [][][]byte {
	sortedMirror := make([][][]byte, 0, len(mirror))
	for k, v := range mirror {
		sortedMirror = append(sortedMirror, [][]byte{[]byte(k), []byte(v)})
	}
	sort.Slice(sortedMirror, func(i, j int) bool {
		return string(sortedMirror[i][0]) < string(sortedMirror[j][0])
	})
	return sortedMirror
}

func TestDifferenceIterator_Basic(t *testing.T) {

	config := &iteratorTestConfig{
		startByteToSet: 'a',
		endByteToSet:   'z',
		startIterate:   nil,
		endIterate:     nil,
		ascending:      true,
	}

	tree, mirrorA := getRandomizedTreeAndMirror(t)
	_, vA, err := tree.SaveVersion()
	require.NoError(t, err)

	// copy the mirror and make changes
	mirrorB := map[string]string{}
	for k, v := range mirrorA {
		mirrorB[k] = v
	}
	randomizeTreeAndMirror(t, tree, mirrorB)

	_, vB, err := tree.SaveVersion()
	require.NoError(t, err)

	// compute expected set differences
	mirrorBMinusA := map[string]string{}
	for k, valB := range mirrorB {
		valA, has := mirrorA[k]
		if !has || (valA != valB) {
			mirrorBMinusA[k] = valB
		}
	}
	sortedBMinusA := mirrorToSlice(mirrorBMinusA)

	treeA, err := tree.GetImmutable(vA)
	require.NoError(t, err)
	itA := NewIterator(config.startIterate, config.endIterate, config.ascending, treeA)

	treeB, err := tree.GetImmutable(vB)
	require.NoError(t, err)
	itB := NewIterator(config.startIterate, config.endIterate, config.ascending, treeB)

	di := NewDifferenceIterator(itA, itB)
	require.True(t, di.Valid())

	for _, expected := range sortedBMinusA {
		require.True(t, di.Valid())
		require.Equal(t, []byte(expected[0]), di.Key())
		require.Equal(t, []byte(expected[1]), di.Value())
		di.Next()
		require.NoError(t, di.Error())
	}

	// Rebuild A by reversing operations

	di = NewDifferenceIterator(
		NewIterator(config.startIterate, config.endIterate, config.ascending, treeA),
		NewIterator(config.startIterate, config.endIterate, config.ascending, treeB),
	)
	require.True(t, di.Valid())

	diAMinusB := NewDifferenceIterator(
		NewIterator(config.startIterate, config.endIterate, config.ascending, treeB),
		NewIterator(config.startIterate, config.endIterate, config.ascending, treeA),
	)
	require.True(t, diAMinusB.Valid())

	for ; di.Valid(); di.Next() {
		tree.Remove([]byte(di.Key()))
	}
	for ; diAMinusB.Valid(); diAMinusB.Next() {
		tree.Set([]byte(diAMinusB.Key()), []byte(diAMinusB.Value()))
	}

	_, vC, err := tree.SaveVersion()
	require.NoError(t, err)

	itA = NewIterator(config.startIterate, config.endIterate, config.ascending, treeA)

	treeC, err := tree.GetImmutable(vC)
	require.NoError(t, err)
	itC := NewIterator(config.startIterate, config.endIterate, config.ascending, treeC)

	for ; itA.Valid(); itA.Next() {
		require.Equal(t, itA.Key(), itC.Key())
		require.Equal(t, itA.Value(), itC.Value())
		itC.Next()
	}
	require.False(t, itC.Valid())
}

func TestIterator_WithDelete_Full_Ascending_Success(t *testing.T) {
	config := &iteratorTestConfig{
		startByteToSet: 'a',
		endByteToSet:   'z',
		startIterate:   nil,
		endIterate:     nil,
		ascending:      false,
	}

	tree, mirror := getRandomizedTreeAndMirror(t)

	_, _, err := tree.SaveVersion()
	require.NoError(t, err)

	randomizeTreeAndMirror(t, tree, mirror)

	_, _, err = tree.SaveVersion()
	require.NoError(t, err)

	err = tree.DeleteVersion(1)
	require.NoError(t, err)

	latestVersion, err := tree.ndb.getLatestVersion()
	require.NoError(t, err)
	immutableTree, err := tree.GetImmutable(latestVersion)
	require.NoError(t, err)

	// sort mirror for assertion
	sortedMirror := make([][]string, 0, len(mirror))
	for k, v := range mirror {
		sortedMirror = append(sortedMirror, []string{k, v})
	}

	sort.Slice(sortedMirror, func(i, j int) bool {
		return sortedMirror[i][0] > sortedMirror[j][0]
	})

	t.Run("Iterator", func(t *testing.T) {
		itr := NewIterator(config.startIterate, config.endIterate, config.ascending, immutableTree)
		require.True(t, itr.Valid())
		assertIterator(t, itr, sortedMirror, config.ascending)
	})

	t.Run("Fast Iterator", func(t *testing.T) {
		itr := NewFastIterator(config.startIterate, config.endIterate, config.ascending, immutableTree.ndb)
		require.True(t, itr.Valid())
		assertIterator(t, itr, sortedMirror, config.ascending)
	})

	t.Run("Unsaved Fast Iterator", func(t *testing.T) {
		itr := NewUnsavedFastIterator(config.startIterate, config.endIterate, config.ascending, immutableTree.ndb, tree.unsavedFastNodeAdditions, tree.unsavedFastNodeRemovals)
		require.True(t, itr.Valid())
		assertIterator(t, itr, sortedMirror, config.ascending)
	})
}

func iteratorSuccessTest(t *testing.T, config *iteratorTestConfig) {
	performTest := func(t *testing.T, itr dbm.Iterator, mirror [][]string) {
		actualStart, actualEnd := itr.Domain()
		require.Equal(t, config.startIterate, actualStart)
		require.Equal(t, config.endIterate, actualEnd)

		require.NoError(t, itr.Error())

		assertIterator(t, itr, mirror, config.ascending)
	}

	t.Run("Iterator", func(t *testing.T) {
		itr, mirror := setupIteratorAndMirror(t, config)
		require.True(t, itr.Valid())
		performTest(t, itr, mirror)
	})

	t.Run("Fast Iterator", func(t *testing.T) {
		itr, mirror := setupFastIteratorAndMirror(t, config)
		require.True(t, itr.Valid())
		performTest(t, itr, mirror)
	})

	t.Run("Unsaved Fast Iterator", func(t *testing.T) {
		itr, mirror := setupUnsavedFastIterator(t, config)
		require.True(t, itr.Valid())
		performTest(t, itr, mirror)
	})
}

func setupIteratorAndMirror(t *testing.T, config *iteratorTestConfig) (dbm.Iterator, [][]string) {
	tree, err := NewMutableTree(dbm.NewMemDB(), 0, false)
	require.NoError(t, err)

	mirror := setupMirrorForIterator(t, config, tree)
	_, _, err = tree.SaveVersion()
	require.NoError(t, err)

	latestVersion, err := tree.ndb.getLatestVersion()
	require.NoError(t, err)
	immutableTree, err := tree.GetImmutable(latestVersion)
	require.NoError(t, err)

	itr := NewIterator(config.startIterate, config.endIterate, config.ascending, immutableTree)
	return itr, mirror
}

func setupFastIteratorAndMirror(t *testing.T, config *iteratorTestConfig) (dbm.Iterator, [][]string) {
	tree, err := NewMutableTree(dbm.NewMemDB(), 0, false)
	require.NoError(t, err)

	mirror := setupMirrorForIterator(t, config, tree)
	_, _, err = tree.SaveVersion()
	require.NoError(t, err)

	itr := NewFastIterator(config.startIterate, config.endIterate, config.ascending, tree.ndb)
	return itr, mirror
}

func setupUnsavedFastIterator(t *testing.T, config *iteratorTestConfig) (dbm.Iterator, [][]string) {
	tree, err := NewMutableTree(dbm.NewMemDB(), 0, false)
	require.NoError(t, err)

	// For unsaved fast iterator, we would like to test the state where
	// there are saved fast nodes as well as some unsaved additions and removals.
	// So, we split the byte range in half where the first half is saved and the second half is unsaved.
	breakpointByte := (config.endByteToSet + config.startByteToSet) / 2

	firstHalfConfig := *config
	firstHalfConfig.endByteToSet = breakpointByte // exclusive

	secondHalfConfig := *config
	secondHalfConfig.startByteToSet = breakpointByte

	// First half of the mirror
	mirror := setupMirrorForIterator(t, &firstHalfConfig, tree)
	_, _, err = tree.SaveVersion()
	require.NoError(t, err)

	// No unsaved additions or removals should be present after saving
	require.Equal(t, 0, len(tree.unsavedFastNodeAdditions))
	require.Equal(t, 0, len(tree.unsavedFastNodeRemovals))

	// Ensure that there are unsaved additions and removals present
	secondHalfMirror := setupMirrorForIterator(t, &secondHalfConfig, tree)

	require.True(t, len(tree.unsavedFastNodeAdditions) >= len(secondHalfMirror))
	require.Equal(t, 0, len(tree.unsavedFastNodeRemovals))

	// Merge the two halves
	if config.ascending {
		mirror = append(mirror, secondHalfMirror...)
	} else {
		mirror = append(secondHalfMirror, mirror...)
	}

	if len(mirror) > 0 {
		// Remove random keys
		for i := 0; i < len(mirror)/4; i++ {
			randIndex := rand.Intn(len(mirror))
			keyToRemove := mirror[randIndex][0]

			_, removed, err := tree.Remove([]byte(keyToRemove))
			require.NoError(t, err)
			require.True(t, removed)

			mirror = append(mirror[:randIndex], mirror[randIndex+1:]...)
		}
	}

	itr := NewUnsavedFastIterator(config.startIterate, config.endIterate, config.ascending, tree.ndb, tree.unsavedFastNodeAdditions, tree.unsavedFastNodeRemovals)
	return itr, mirror
}
