package kzg

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/params"
	"github.com/protolambda/go-kzg/bls"
)

// KZG CRS for G2
var kzgSetupG2 []bls.G2Point

// KZG CRS for commitment computation
var kzgSetupLagrange []bls.G1Point

// KZG CRS for G1 (only used in tests (for proof creation))
var KzgSetupG1 []bls.G1Point

// Convert polynomial in evaluation form to KZG commitment
func BlobToKzg(eval []bls.Fr) *bls.G1Point {
	return bls.LinCombG1(kzgSetupLagrange, eval)
}

// Verify a KZG proof
func VerifyKzgProof(commitment *bls.G1Point, x *bls.Fr, y *bls.Fr, proof *bls.G1Point) bool {
	// Verify the pairing equation
	var xG2 bls.G2Point
	bls.MulG2(&xG2, &bls.GenG2, x)
	var sMinuxX bls.G2Point
	bls.SubG2(&sMinuxX, &kzgSetupG2[1], &xG2)
	var yG1 bls.G1Point
	bls.MulG1(&yG1, &bls.GenG1, y)
	var commitmentMinusY bls.G1Point
	bls.SubG1(&commitmentMinusY, commitment, &yG1)

	return bls.PairingsVerify(&commitmentMinusY, &bls.GenG2, proof, &sMinuxX)
}

type BlobsBatch struct {
	sync.Mutex
	init                bool
	aggregateCommitment bls.G1Point
	aggregateBlob       [params.FieldElementsPerBlob]bls.Fr
}

func (batch *BlobsBatch) Join(commitments []*bls.G1Point, blobs [][]bls.Fr) error {
	batch.Lock()
	defer batch.Unlock()
	if len(commitments) != len(blobs) {
		return fmt.Errorf("expected commitments len %d to equal blobs len %d", len(commitments), len(blobs))
	}
	if !batch.init && len(commitments) > 0 {
		batch.init = true
		bls.CopyG1(&batch.aggregateCommitment, commitments[0])
		copy(batch.aggregateBlob[:], blobs[0])
		commitments = commitments[1:]
		blobs = blobs[1:]
	}
	for i, commit := range commitments {
		batch.join(commit, blobs[i])
	}
	return nil
}

func (batch *BlobsBatch) join(commitment *bls.G1Point, blob []bls.Fr) {
	// we multiply the input we are joining with a random scalar, so we can add it to the aggregate safely
	randomScalar := bls.RandomFr()

	// TODO: instead of computing the lin-comb of the commitments on the go, we could buffer
	// the random scalar and commitment, and run a LinCombG1 over all of them during Verify()
	var tmpG1 bls.G1Point
	bls.MulG1(&tmpG1, commitment, randomScalar)
	bls.AddG1(&batch.aggregateCommitment, &batch.aggregateCommitment, &tmpG1)

	var tmpFr bls.Fr
	for i := 0; i < params.FieldElementsPerBlob; i++ {
		bls.MulModFr(&tmpFr, &blob[i], randomScalar)
		bls.AddModFr(&batch.aggregateBlob[i], &batch.aggregateBlob[i], &tmpFr)
	}
}

func (batch *BlobsBatch) Verify() error {
	batch.Lock()
	defer batch.Unlock()
	if !batch.init {
		return nil // empty batch
	}
	// Compute both MSMs and check equality
	lResult := bls.LinCombG1(kzgSetupLagrange, batch.aggregateBlob[:])
	if !bls.EqualG1(lResult, &batch.aggregateCommitment) {
		return errors.New("BlobsBatch failed to Verify")
	}
	return nil
}

// Verify that the list of `commitments` maps to the list of `blobs`
//
// This is an optimization over the naive approach (found in the EIP) of iteratively checking each blob against each
// commitment.  The naive approach requires n*l scalar multiplications where `n` is the number of blobs and `l` is
// FIELD_ELEMENTS_PER_BLOB to compute the commitments for all blobs.
//
// A more efficient approach is to build a linear combination of all blobs and commitments and check all of them in a
// single multi-scalar multiplication.
//
// The MSM would look like this (for three blobs with two field elements each):
//     r_0(b0_0*L_0 + b0_1*L_1) + r_1(b1_0*L_0 + b1_1*L_1) + r_2(b2_0*L_0 + b2_1*L_1)
// which we would need to check against the linear combination of commitments: r_0*C_0 + r_1*C_1 + r_2*C_2
// In the above, `r` are the random scalars of the linear combination, `b0` is the zero blob, `L` are the elements
// of the KZG_SETUP_LAGRANGE and `C` are the commitments provided.
//
// By regrouping the above equation around the `L` points we can reduce the length of the MSM further
// (down to just `n` scalar multiplications) by making it look like this:
//     (r_0*b0_0 + r_1*b1_0 + r_2*b2_0) * L_0 + (r_0*b0_1 + r_1*b1_1 + r_2*b2_1) * L_1
func VerifyBlobs(commitments []*bls.G1Point, blobs [][]bls.Fr) error {
	// Prepare objects to hold our two MSMs
	lPoints := make([]bls.G1Point, params.FieldElementsPerBlob)
	lScalars := make([]bls.Fr, params.FieldElementsPerBlob)
	rPoints := make([]bls.G1Point, len(commitments))
	rScalars := make([]bls.Fr, len(commitments))

	// Generate list of random scalars for lincomb
	rList := make([]bls.Fr, len(blobs))
	for i := 0; i < len(blobs); i++ {
		bls.CopyFr(&rList[i], bls.RandomFr())
	}

	// Build left-side MSM:
	//   (r_0*b0_0 + r_1*b1_0 + r_2*b2_0) * L_0 + (r_0*b0_1 + r_1*b1_1 + r_2*b2_1) * L_1
	for c := 0; c < params.FieldElementsPerBlob; c++ {
		var sum bls.Fr
		for i := 0; i < len(blobs); i++ {
			var tmp bls.Fr

			r := rList[i]
			blob := blobs[i]

			bls.MulModFr(&tmp, &r, &blob[c])
			bls.AddModFr(&sum, &sum, &tmp)
		}
		lScalars[c] = sum
		lPoints[c] = kzgSetupLagrange[c]
	}

	// Build right-side MSM: r_0 * C_0 + r_1 * C_1 + r_2 * C_2 + ...
	for i, commitment := range commitments {
		rScalars[i] = rList[i]
		rPoints[i] = *commitment
	}

	// Compute both MSMs and check equality
	lResult := bls.LinCombG1(lPoints, lScalars)
	rResult := bls.LinCombG1(rPoints, rScalars)
	if !bls.EqualG1(lResult, rResult) {
		return errors.New("VerifyBlobs failed")
	}

	// TODO: Potential improvement is to unify both MSMs into a single MSM, but you would need to batch-invert the `r`s
	// of the right-side MSM to effectively pull them to the left side.

	return nil
}

type JSONTrustedSetup struct {
	SetupG1       []bls.G1Point
	SetupG2       []bls.G2Point
	SetupLagrange []bls.G1Point
}

// Initialize KZG subsystem (load the trusted setup data)
func init() {
	var parsedSetup = JSONTrustedSetup{}

	// TODO: This is dirty. KZG setup should be loaded using an actual config file directive
	err := json.Unmarshal([]byte(KZGSetupStr), &parsedSetup)
	if err != nil {
		panic(err)
	}

	kzgSetupG2 = parsedSetup.SetupG2
	kzgSetupLagrange = parsedSetup.SetupLagrange
	KzgSetupG1 = parsedSetup.SetupG1
}
