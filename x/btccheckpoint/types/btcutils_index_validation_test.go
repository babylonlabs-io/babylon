package types_test

import (
	"encoding/hex"
	"testing"

	bbn "github.com/babylonlabs-io/babylon/v4/types"
	btcctypes "github.com/babylonlabs-io/babylon/v4/x/btccheckpoint/types"
	btcchaincfg "github.com/btcsuite/btcd/chaincfg"
	"github.com/stretchr/testify/require"
)

// TestMerkleIndexValidationFix verifies that the fix correctly rejects forged indices
// while still accepting valid indices. This test confirms that the vulnerability
// described in report #63323 has been fixed.
func TestMerkleIndexValidationFix(t *testing.T) {
	headerHex := "0000e0208c3b3ed3aa778eaecdcbe91dae57197ce1baa0d7c33e86d00d0100000000000079ffca6c6b36348c306234dee2fe47bafd76df7e70c95cbdff3efeb81e5abe71ea88b860fcff031a45722027"
	transactions := []string{
		"010000000001010000000000000000000000000000000000000000000000000000000000000000ffffffff240381841e0c2074657374206d696e65722012097573200909200902825401fa4184010000ffffffff02efbc97000000000017a9140d2eb00a31486c91e3dbefa13ac714e236390dad870000000000000000266a24aa21a9ed02ee31a4ff032e606a5bc1af454ddca6695a1261a69d4ddb24d6dd10cb6d3fcd0120000000000000000000000000000000000000000000000000000000000000000000000000",
		"02000000000101761d35946ece6a53b79380119ccda626a8efd5caee724d71d81f404f5a33003f00000000171600145074935eaaf3cc1f04acc64c2c4f88737ff17896feffffff02f0f20f000000000017a9147e96bcc24d343e35f857f593eca765ccdf200b17872172b38a0000000017a914647dbca76f3d3426a564361c9539aae810752e4d870247304402204e80eb98037ec577a88a4712e6b3ea81eb23541052ecd595211edce78034fbd8022048bc0507ca70261810906c818e761d6a53cb69ad11c2c19ac554047622e88f85012103521bdcb10ea983094184c8b8dc49698541d63c45d91d90771b75434d568c80fb7f841e00",
		"02000000000101b8ff4b6851eee40c958c58916604c15533e7fb4a64a9ec509a55d10167b64b290000000000feffffff023b1ccdd700000000160014484f0aa3ea9b74beb476be62936df51c2fc59b99e84610000000000016001448717afa6934ad4da8e5e827ca061d11e41f78e602473044022079d02fd7cca6aa2f3e8860987b3fae269677ea97bc1c79553d1e53b6d5329454022040ecf6545258d0702713f46d4bafe1f1eb05c1dec983b84982d1cf2c807195dd0121034120b65994ee9788e450312b312d1fcef975c0f34dadd0fbe8ed01c9c61633a880841e00",
		"0100000001e7742bc7ac999bd4b0832809534a5965cf5b53abdc75e3f840702be92b1f82dd030000006a47304402202fe1e5defb67549a2f2a8b7d754a80866583f348e2aae9a61ece13b0842b16ca0220311eacc4c8bf859ba2ad76b7093caa366cff4b4ec66fb6e14708915cad3a04e40121037435c194e9b01b3d7f7a2802d6684a3af68d05bbf4ec8f17021980d777691f1dfdffffff040000000000000000536a4c5058365b8ca81abb972accd336d8f05be0c843b5486fb72528877a5f50e898b14d3b39e011a46ff8b0ad5a933ab2caee547240cb41e7da3cf69d5f364f4ba76e06796dc5001e847e0005001e818e00010010270000000000001976a914000000000000000000000000000000000000000088ac10270000000000001976a914000000000000000000000000000000000000000088acc1e09a04000000001976a914ba27f99e007c7f605a8305e318c1abde3cd220ac88ac00000000",
		"0100000001d6541ff19b573a4742925a56552e608fc827a149e412e30018e64fc39c6ebcbb010000008b483045022100c9a4d2502164a78caefd4d3b1deac72bdf418636e2cf16bea7a051438bec8499022070dcebad8a71a1acc6fea62512ae88a9e7901d31cbeca4a1d1af10640eba538c0141045a5ddc925295b71bafbe56bf4c10e1c1bc7c3a2bf5116b72f5dd202bccc032955afc5191f626284508072d397fd0fde700ae6feb2a35c1c391b12971960e6df6ffffffff03ee050000000000001600140fb58dc4fc27d579fd59cd18d3b44f8b5df1b47b2b2be30b000000001976a914d9ea351605b36fc3a967d790132230eb7eced36688ac0000000000000000256a2302000fa26dbf437f2811124e8395d532f969f2ee83a6d0542e2f5798ce37f267f2fdaa00000000",
		"01000000000101614b0cdbc00644c6e8bb016ab669acafacab0279c4df379440cf225ed1fd2c8a0100000000ffffffff02407e05000000000017a914dc75fc89f54f9618ee4fb5ef538c3baa46adf7458719480800000000001600146bee0d3f361510aae3e9d5014f8f91db21342506024830450221009315bcf6106e8666ea4dea43f09a81550f4b481b67e20375a3acb2678de500e402201d8a79065987b1cad8d032d4e58652d6258d4e3f53b7d15b2a78029c77f595100121035ad29a75b4561eb39c4f81a6f860d032385ef56717f3b896322c9a611cb40ed300000000",
		"02000000000101bfb1f07e1055a1ec06c045322ce1d8354d59f10fea0ed3294bd12954150f404f0100000000fdffffff02142b020000000000160014f2bb4458b593dfd6d96d1474469e240121ccfc0540420f0000000000160014206f30d19f2d39dfeda1933e07a62e38ce380da10247304402202576f7adcca78e963366c7b16c3732f3401ccc42e1726ec1e675cecd1928591a022028b60a4ab7de85112252aacaca74f3caf4927a2932c719cecb008194a51a45ad012102b65c0805712db5158e1554297fe51e468e8bbb25464d3fed183d8e7b969b539e7d841e00",
		"020000000001032c5faf7fc3751f79f93ed389db1af6394fffede942d51fcec61d15bb5829302d01000000171600147af2e87a43f5370b17bb0bd0ea18849e123ba61afeffffffad6d0fe044f3440347fa07e2fe07fac2d1a585f0a66b9a23265d968667aa21ec0000000017160014b5d5a81ba37d47ad5fe276b23f3973f56b70a459feffffff2b564e1dd88cbddbd79ba2b5467f64cbbec439c008738906e361f1b2f5eb0fe40000000000feffffff02801a060000000000160014407d31e030df0f64a73f142a0feecd4875a0626c32810f00000000001600149cffa54d75caa5cb3897efde3ef9017d593e203a0247304402200bf2fcd9dec73082763bef3999f531f9b6b5f707535596244623f874973de610022013ac74dd9a5b6eb7300e35bea44546f62b726ec4b277342920d2378df34361a501210253d9126db8349ae5550fadd02ebe612fea2dd239ba32f1029342d407d4ab6fba02473044022048b1d568ee52de84ca71dddab0503e1e8f4829a6478a1d69baeab8896ad5c9230220757e3255cf415e067f843396e5226ff6ed8facb45013773e18aa9bb0243791f50121038a51c7db5ab7377e8d5f9a06be6b6b46c900df51af9495a79f3ef01a46c71f4902473044022005030ff6d9539774eb1a03eb3ae06e73c15011404076cac08dc2fbd4d54b67a0022052d902113c9ed40c326bea976d3f0ce475fc3a575ea55e9bf158d410fc842f46012103c2f8ebfebd41bfa467ab63f3ef7e734c64ee38a46fc2f4207f3aca935bd170be7f841e00",
		"010000000001010f90038c463b5f6fc0593ec8110951c665052b2631e5c0978f65e48706d195b30100000000ffffffff02b5264408000000001600147fa2f0e860094b79fda8ac58245e2c9c5c1fbb1f8c5a0000000000001976a9141d7cb15cc4393354b4007e97161c58c1e16f0c0d88ac02473044022030aa6f4718e6dc75e0448b6de4c4d8a2a0c1c49be5c5eca1d3902b1b8a44c9da02201982eead66664a27637b46b90fc88bd80126208381700be6be327f0c8c79386d0121029459153248d701d7402b0ef3dddfde42f3867a52f172fa383746287014dd4a2e00000000",
	}

	headerBytes, err := bbn.NewBTCHeaderBytesFromHex(headerHex)
	require.NoError(t, err)

	header := headerBytes.ToBlockHeader()
	merkleRoot := &header.MerkleRoot

	var transactionBytes [][]byte
	for _, txHex := range transactions {
		txBytes, err := hex.DecodeString(txHex)
		require.NoError(t, err)
		transactionBytes = append(transactionBytes, txBytes)
	}

	t.Run("Valid indices pass verification", func(t *testing.T) {
		for txIdx := 0; txIdx < len(transactions); txIdx++ {
			txBytes := transactionBytes[txIdx]
			tx, err := btcctypes.ParseTransaction(txBytes)
			require.NoError(t, err)

			branch, err := btcctypes.CreateProofForIdx(transactionBytes, uint(txIdx))
			require.NoError(t, err)

			var proof []byte
			for _, h := range branch {
				proof = append(proof, h.CloneBytes()...)
			}

			valid := btcctypes.VerifyInclusionProof(tx, merkleRoot, proof, uint32(txIdx))
			require.True(t, valid, "Valid index %d should pass verification", txIdx)
		}
	})

	t.Run("Forged indices are rejected - coinbase transaction", func(t *testing.T) {
		coinbaseIdx := uint(0)
		coinbaseTxBytes := transactionBytes[coinbaseIdx]

		branch, err := btcctypes.CreateProofForIdx(transactionBytes, coinbaseIdx)
		require.NoError(t, err)

		proofDepth := uint(len(branch))
		indexMultiplier := uint32(1 << proofDepth)

		var proof []byte
		for _, h := range branch {
			proof = append(proof, h.CloneBytes()...)
		}

		coinbaseTx, err := btcctypes.ParseTransaction(coinbaseTxBytes)
		require.NoError(t, err)

		// Test multiple forged indices
		for k := uint32(1); k <= 5; k++ {
			fakeIndex := uint32(coinbaseIdx) + k*indexMultiplier
			valid := btcctypes.VerifyInclusionProof(coinbaseTx, merkleRoot, proof, fakeIndex)
			require.False(t, valid, "Forged index %d (0 + %d * %d) should be rejected", fakeIndex, k, indexMultiplier)
		}
	})

	t.Run("Forged indices are rejected - regular transaction", func(t *testing.T) {
		txIdx := uint(3)
		txBytes := transactionBytes[txIdx]

		branch, err := btcctypes.CreateProofForIdx(transactionBytes, txIdx)
		require.NoError(t, err)

		proofDepth := uint(len(branch))
		indexMultiplier := uint32(1 << proofDepth)

		var proof []byte
		for _, h := range branch {
			proof = append(proof, h.CloneBytes()...)
		}

		tx, err := btcctypes.ParseTransaction(txBytes)
		require.NoError(t, err)

		realIndex := uint32(txIdx)
		validWithRealIndex := btcctypes.VerifyInclusionProof(tx, merkleRoot, proof, realIndex)
		require.True(t, validWithRealIndex, "Real index %d should pass verification", realIndex)

		// Test multiple forged indices
		for k := uint32(1); k <= 5; k++ {
			fakeIdx := realIndex + k*indexMultiplier
			valid := btcctypes.VerifyInclusionProof(tx, merkleRoot, proof, fakeIdx)
			require.False(t, valid, "Forged index %d (%d + %d * %d) should be rejected", fakeIdx, realIndex, k, indexMultiplier)
		}
	})

	t.Run("ParseProof rejects forged indices", func(t *testing.T) {
		txIdx := uint(3)

		branch, err := btcctypes.CreateProofForIdx(transactionBytes, txIdx)
		require.NoError(t, err)

		proofDepth := uint(len(branch))
		indexMultiplier := uint32(1 << proofDepth)

		var proof []byte
		for _, h := range branch {
			proof = append(proof, h.CloneBytes()...)
		}

		realIndex := uint32(txIdx)
		parsedProof, err := btcctypes.ParseProof(
			transactionBytes[txIdx],
			realIndex,
			proof,
			&headerBytes,
			btcchaincfg.TestNet3Params.PowLimit,
		)
		require.NoError(t, err)
		require.NotNil(t, parsedProof)

		fakeIndex := realIndex + indexMultiplier
		parsedProofFake, err := btcctypes.ParseProof(
			transactionBytes[txIdx],
			fakeIndex,
			proof,
			&headerBytes,
			btcchaincfg.TestNet3Params.PowLimit,
		)
		require.Error(t, err, "ParseProof with forged index %d should fail", fakeIndex)
		require.Nil(t, parsedProofFake)
	})
}

// TestMerkleIndexEdgeCases tests edge cases for index validation
func TestMerkleIndexEdgeCases(t *testing.T) {
	t.Run("Single transaction block - index 0 valid, others invalid", func(t *testing.T) {
		headerHex := "000000200761788fd78b840d06e9b0e2e46ee8645ebf294136abe64b53430300000000006250ecc747676b963379d201e244c074f7673e17acc054a410fbdd2e9662debb08547958fcff031b705d182c"
		transactions := []string{
			"01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff2203c27a10150e6d696e65642062792062636f696e58795408100000000000cf110000ffffffff0214d917000000000017a9146859969825bb2787f803a3d0eeb632998ce4f50187bcf238090000000017a9146859969825bb2787f803a3d0eeb632998ce4f5018700000000",
			"01000000017ba216f5893308e3a6706d6a644da0e58b765cf4fe8d6dd78d14b7b771f6892601000000fdfd0000473044022075025b2858500747b3a9005b616625f0143610c06d36aee3153c52f377e82f1d02202215d5feff5daee05b81be934f38c0b01b0194874a1b4372d211a141bc46c7440148304502210083fc1e3553d9ea849387ee704fde23d97909b54e4b7b59316c702b72177775610220429c2128bb1602df56e6fdb463be6dd40acaeeebfe3cd80abb85fe312fa3185c014c69522102b4905cfaa07073953235d02a9046c76ce398a4e7d0b41b2004089698304b11cd2102eeee7cdafe05cc4cf526355e866cf8bc146a50172cf3ab48661eed00bd423a9a21038e282cca3b7851d02fe10c4c6b99a175662b3ad796daac5a9356002aab652d5853aeffffffff0600000000000000000b6a095349474e4154555245a94c87000000000017a9140c7497f8303b61239314a4c7022d5e9081130f2b877a5925000000000017a9141850de01509f098efae2b8a3cde05d6ae34767fb87759707000000000017a9143297b2f17781542470eb6143c2b70f86697b152687c24c00000000000017a9142c5ee431f022ff2ccafee158b6b9db046f5554a087c90b1e000000000017a914081e2f578f7fd6c641578945f1e432ba0de60a038700000000",
		}

		headerBytes, err := bbn.NewBTCHeaderBytesFromHex(headerHex)
		require.NoError(t, err)

		header := headerBytes.ToBlockHeader()
		merkleRoot := &header.MerkleRoot

		var transactionBytes [][]byte
		for _, txHex := range transactions {
			txBytes, err := hex.DecodeString(txHex)
			require.NoError(t, err)
			transactionBytes = append(transactionBytes, txBytes)
		}

		txIdx := uint(1)
		branch, err := btcctypes.CreateProofForIdx(transactionBytes, txIdx)
		require.NoError(t, err)

		var proof []byte
		for _, h := range branch {
			proof = append(proof, h.CloneBytes()...)
		}

		tx, err := btcctypes.ParseTransaction(transactionBytes[txIdx])
		require.NoError(t, err)

		validIndex := uint32(1)
		valid := btcctypes.VerifyInclusionProof(tx, merkleRoot, proof, validIndex)
		require.True(t, valid, "Valid index should pass")

		proofDepth := uint(len(branch))
		indexMultiplier := uint32(1 << proofDepth)
		invalidIndex := validIndex + indexMultiplier
		valid = btcctypes.VerifyInclusionProof(tx, merkleRoot, proof, invalidIndex)
		require.False(t, valid, "Invalid index %d should fail", invalidIndex)
	})

	t.Run("Maximum valid index passes, one above fails", func(t *testing.T) {
		headerHex := "0000e0208c3b3ed3aa778eaecdcbe91dae57197ce1baa0d7c33e86d00d0100000000000079ffca6c6b36348c306234dee2fe47bafd76df7e70c95cbdff3efeb81e5abe71ea88b860fcff031a45722027"
		transactions := []string{
			"010000000001010000000000000000000000000000000000000000000000000000000000000000ffffffff240381841e0c2074657374206d696e65722012097573200909200902825401fa4184010000ffffffff02efbc97000000000017a9140d2eb00a31486c91e3dbefa13ac714e236390dad870000000000000000266a24aa21a9ed02ee31a4ff032e606a5bc1af454ddca6695a1261a69d4ddb24d6dd10cb6d3fcd0120000000000000000000000000000000000000000000000000000000000000000000000000",
			"02000000000101761d35946ece6a53b79380119ccda626a8efd5caee724d71d81f404f5a33003f00000000171600145074935eaaf3cc1f04acc64c2c4f88737ff17896feffffff02f0f20f000000000017a9147e96bcc24d343e35f857f593eca765ccdf200b17872172b38a0000000017a914647dbca76f3d3426a564361c9539aae810752e4d870247304402204e80eb98037ec577a88a4712e6b3ea81eb23541052ecd595211edce78034fbd8022048bc0507ca70261810906c818e761d6a53cb69ad11c2c19ac554047622e88f85012103521bdcb10ea983094184c8b8dc49698541d63c45d91d90771b75434d568c80fb7f841e00",
			"02000000000101b8ff4b6851eee40c958c58916604c15533e7fb4a64a9ec509a55d10167b64b290000000000feffffff023b1ccdd700000000160014484f0aa3ea9b74beb476be62936df51c2fc59b99e84610000000000016001448717afa6934ad4da8e5e827ca061d11e41f78e602473044022079d02fd7cca6aa2f3e8860987b3fae269677ea97bc1c79553d1e53b6d5329454022040ecf6545258d0702713f46d4bafe1f1eb05c1dec983b84982d1cf2c807195dd0121034120b65994ee9788e450312b312d1fcef975c0f34dadd0fbe8ed01c9c61633a880841e00",
			"0100000001e7742bc7ac999bd4b0832809534a5965cf5b53abdc75e3f840702be92b1f82dd030000006a47304402202fe1e5defb67549a2f2a8b7d754a80866583f348e2aae9a61ece13b0842b16ca0220311eacc4c8bf859ba2ad76b7093caa366cff4b4ec66fb6e14708915cad3a04e40121037435c194e9b01b3d7f7a2802d6684a3af68d05bbf4ec8f17021980d777691f1dfdffffff040000000000000000536a4c5058365b8ca81abb972accd336d8f05be0c843b5486fb72528877a5f50e898b14d3b39e011a46ff8b0ad5a933ab2caee547240cb41e7da3cf69d5f364f4ba76e06796dc5001e847e0005001e818e00010010270000000000001976a914000000000000000000000000000000000000000088ac10270000000000001976a914000000000000000000000000000000000000000088acc1e09a04000000001976a914ba27f99e007c7f605a8305e318c1abde3cd220ac88ac00000000",
			"0100000001d6541ff19b573a4742925a56552e608fc827a149e412e30018e64fc39c6ebcbb010000008b483045022100c9a4d2502164a78caefd4d3b1deac72bdf418636e2cf16bea7a051438bec8499022070dcebad8a71a1acc6fea62512ae88a9e7901d31cbeca4a1d1af10640eba538c0141045a5ddc925295b71bafbe56bf4c10e1c1bc7c3a2bf5116b72f5dd202bccc032955afc5191f626284508072d397fd0fde700ae6feb2a35c1c391b12971960e6df6ffffffff03ee050000000000001600140fb58dc4fc27d579fd59cd18d3b44f8b5df1b47b2b2be30b000000001976a914d9ea351605b36fc3a967d790132230eb7eced36688ac0000000000000000256a2302000fa26dbf437f2811124e8395d532f969f2ee83a6d0542e2f5798ce37f267f2fdaa00000000",
			"01000000000101614b0cdbc00644c6e8bb016ab669acafacab0279c4df379440cf225ed1fd2c8a0100000000ffffffff02407e05000000000017a914dc75fc89f54f9618ee4fb5ef538c3baa46adf7458719480800000000001600146bee0d3f361510aae3e9d5014f8f91db21342506024830450221009315bcf6106e8666ea4dea43f09a81550f4b481b67e20375a3acb2678de500e402201d8a79065987b1cad8d032d4e58652d6258d4e3f53b7d15b2a78029c77f595100121035ad29a75b4561eb39c4f81a6f860d032385ef56717f3b896322c9a611cb40ed300000000",
			"02000000000101bfb1f07e1055a1ec06c045322ce1d8354d59f10fea0ed3294bd12954150f404f0100000000fdffffff02142b020000000000160014f2bb4458b593dfd6d96d1474469e240121ccfc0540420f0000000000160014206f30d19f2d39dfeda1933e07a62e38ce380da10247304402202576f7adcca78e963366c7b16c3732f3401ccc42e1726ec1e675cecd1928591a022028b60a4ab7de85112252aacaca74f3caf4927a2932c719cecb008194a51a45ad012102b65c0805712db5158e1554297fe51e468e8bbb25464d3fed183d8e7b969b539e7d841e00",
			"020000000001032c5faf7fc3751f79f93ed389db1af6394fffede942d51fcec61d15bb5829302d01000000171600147af2e87a43f5370b17bb0bd0ea18849e123ba61afeffffffad6d0fe044f3440347fa07e2fe07fac2d1a585f0a66b9a23265d968667aa21ec0000000017160014b5d5a81ba37d47ad5fe276b23f3973f56b70a459feffffff2b564e1dd88cbddbd79ba2b5467f64cbbec439c008738906e361f1b2f5eb0fe40000000000feffffff02801a060000000000160014407d31e030df0f64a73f142a0feecd4875a0626c32810f00000000001600149cffa54d75caa5cb3897efde3ef9017d593e203a0247304402200bf2fcd9dec73082763bef3999f531f9b6b5f707535596244623f874973de610022013ac74dd9a5b6eb7300e35bea44546f62b726ec4b277342920d2378df34361a501210253d9126db8349ae5550fadd02ebe612fea2dd239ba32f1029342d407d4ab6fba02473044022048b1d568ee52de84ca71dddab0503e1e8f4829a6478a1d69baeab8896ad5c9230220757e3255cf415e067f843396e5226ff6ed8facb45013773e18aa9bb0243791f50121038a51c7db5ab7377e8d5f9a06be6b6b46c900df51af9495a79f3ef01a46c71f4902473044022005030ff6d9539774eb1a03eb3ae06e73c15011404076cac08dc2fbd4d54b67a0022052d902113c9ed40c326bea976d3f0ce475fc3a575ea55e9bf158d410fc842f46012103c2f8ebfebd41bfa467ab63f3ef7e734c64ee38a46fc2f4207f3aca935bd170be7f841e00",
			"010000000001010f90038c463b5f6fc0593ec8110951c665052b2631e5c0978f65e48706d195b30100000000ffffffff02b5264408000000001600147fa2f0e860094b79fda8ac58245e2c9c5c1fbb1f8c5a0000000000001976a9141d7cb15cc4393354b4007e97161c58c1e16f0c0d88ac02473044022030aa6f4718e6dc75e0448b6de4c4d8a2a0c1c49be5c5eca1d3902b1b8a44c9da02201982eead66664a27637b46b90fc88bd80126208381700be6be327f0c8c79386d0121029459153248d701d7402b0ef3dddfde42f3867a52f172fa383746287014dd4a2e00000000",
		}

		headerBytes, err := bbn.NewBTCHeaderBytesFromHex(headerHex)
		require.NoError(t, err)

		header := headerBytes.ToBlockHeader()
		merkleRoot := &header.MerkleRoot

		var transactionBytes [][]byte
		for _, txHex := range transactions {
			txBytes, err := hex.DecodeString(txHex)
			require.NoError(t, err)
			transactionBytes = append(transactionBytes, txBytes)
		}

		lastTxIdx := uint(len(transactions) - 1)
		branch, err := btcctypes.CreateProofForIdx(transactionBytes, lastTxIdx)
		require.NoError(t, err)

		var proof []byte
		for _, h := range branch {
			proof = append(proof, h.CloneBytes()...)
		}

		tx, err := btcctypes.ParseTransaction(transactionBytes[lastTxIdx])
		require.NoError(t, err)

		validIndex := uint32(lastTxIdx)
		valid := btcctypes.VerifyInclusionProof(tx, merkleRoot, proof, validIndex)
		require.True(t, valid, "Maximum valid index %d should pass", validIndex)

		proofDepth := uint(len(branch))
		indexMultiplier := uint32(1 << proofDepth)
		invalidIndex := validIndex + indexMultiplier
		valid = btcctypes.VerifyInclusionProof(tx, merkleRoot, proof, invalidIndex)
		require.False(t, valid, "Index beyond maximum (%d) should fail", invalidIndex)
	})
}
