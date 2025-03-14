/*
Copyright 2020 IBM All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package client

import (
	"context"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/hyperledger/fabric-gateway/pkg/internal/test"
	"github.com/hyperledger/fabric-protos-go-apiv2/gateway"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/runtime/protoiface"
)

func NewStatusError(t *testing.T, code codes.Code, message string, details ...protoiface.MessageV1) error {
	s, err := status.New(code, message).WithDetails(details...)
	require.NoError(t, err)

	return s.Err()
}

func TestEvaluateTransaction(t *testing.T) {
	newEvaluateResponse := func(value []byte) *gateway.EvaluateResponse {
		return &gateway.EvaluateResponse{
			Result: &peer.Response{
				Payload: []byte(value),
			},
		}
	}

	t.Run("Returns evaluate error", func(t *testing.T) {
		expected := NewStatusError(t, codes.Aborted, "EVALUATE_ERROR")
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Return(nil, expected)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

		_, err := contract.EvaluateTransaction("transaction")

		require.ErrorIs(t, err, expected, "error type: %T", err)
		require.ErrorContains(t, err, expected.Error(), "message")
		require.Equal(t, status.Code(expected), status.Code(err), "status code")
	})

	for name, testCase := range map[string]struct {
		run func(*testing.T, *Contract) ([]byte, error)
	}{
		"EvaluateTransaction returns result": {
			run: func(t *testing.T, contract *Contract) ([]byte, error) {
				return contract.EvaluateTransaction("transaction")
			},
		},
		"EvaluateWithContext returns result": {
			run: func(t *testing.T, contract *Contract) ([]byte, error) {
				return contract.EvaluateWithContext(context.Background(), "transaction")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			expected := []byte("TRANSACTION_RESULT")
			mockClient := NewMockGatewayClient(gomock.NewController(t))
			mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
				Return(newEvaluateResponse(expected), nil)

			contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

			actual, err := testCase.run(t, contract)
			require.NoError(t, err)

			require.EqualValues(t, expected, actual)
		})
	}
	t.Run("Includes channel name in proposal", func(t *testing.T) {
		var actual string
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, in *gateway.EvaluateRequest, _ ...grpc.CallOption) {
				actual = test.AssertUnmarshalChannelheader(t, in.ProposedTransaction).ChannelId
			}).
			Return(newEvaluateResponse(nil), nil).
			Times(1)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

		_, err := contract.EvaluateTransaction("transaction")
		require.NoError(t, err)

		expected := contract.channelName
		require.Equal(t, expected, actual)
	})

	t.Run("Includes chaincode name in proposal", func(t *testing.T) {
		var actual string
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, in *gateway.EvaluateRequest, _ ...grpc.CallOption) {
				actual = test.AssertUnmarshalInvocationSpec(t, in.ProposedTransaction).ChaincodeSpec.ChaincodeId.Name
			}).
			Return(newEvaluateResponse(nil), nil).
			Times(1)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

		_, err := contract.EvaluateTransaction("transaction")
		require.NoError(t, err)

		expected := contract.chaincodeName
		require.Equal(t, expected, actual)
	})

	t.Run("Includes transaction name in proposal for default smart contract", func(t *testing.T) {
		var args [][]byte
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, in *gateway.EvaluateRequest, _ ...grpc.CallOption) {
				args = test.AssertUnmarshalInvocationSpec(t, in.ProposedTransaction).ChaincodeSpec.Input.Args
			}).
			Return(newEvaluateResponse(nil), nil).
			Times(1)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

		expected := "TRANSACTION_NAME"
		_, err := contract.EvaluateTransaction(expected)
		require.NoError(t, err)

		actual := string(args[0])
		require.Equal(t, expected, actual, "got Args: %s", args)
	})

	t.Run("Includes transaction name in proposal for named smart contract", func(t *testing.T) {
		var args [][]byte
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, in *gateway.EvaluateRequest, _ ...grpc.CallOption) {
				args = test.AssertUnmarshalInvocationSpec(t, in.ProposedTransaction).ChaincodeSpec.Input.Args
			}).
			Return(newEvaluateResponse(nil), nil).
			Times(1)

		contract := AssertNewTestContractWithName(t, "chaincode", "CONTRACT_NAME", WithGatewayClient(mockClient))

		_, err := contract.EvaluateTransaction("TRANSACTION_NAME")
		require.NoError(t, err)

		actual := string(args[0])
		expected := "CONTRACT_NAME:TRANSACTION_NAME"
		require.Equal(t, expected, actual, "got Args: %s", args)
	})

	t.Run("Includes arguments in proposal", func(t *testing.T) {
		var args [][]byte
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, in *gateway.EvaluateRequest, _ ...grpc.CallOption) {
				args = test.AssertUnmarshalInvocationSpec(t, in.ProposedTransaction).ChaincodeSpec.Input.Args
			}).
			Return(newEvaluateResponse(nil), nil).
			Times(1)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

		expected := []string{"one", "two", "three"}
		_, err := contract.EvaluateTransaction("transaction", expected...)
		require.NoError(t, err)

		actual := bytesAsStrings(args[1:])
		require.EqualValues(t, expected, actual, "got Args: %s", args)
	})

	t.Run("Includes channel name in proposed transaction", func(t *testing.T) {
		var actual string
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, in *gateway.EvaluateRequest, _ ...grpc.CallOption) {
				actual = in.ChannelId
			}).
			Return(newEvaluateResponse(nil), nil).
			Times(1)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

		_, err := contract.EvaluateTransaction("transaction")
		require.NoError(t, err)

		expected := contract.channelName
		require.Equal(t, expected, actual)
	})

	t.Run("Includes transaction ID in proposed transaction", func(t *testing.T) {
		var actual string
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, in *gateway.EvaluateRequest, _ ...grpc.CallOption) {
				actual = test.AssertUnmarshalChannelheader(t, in.ProposedTransaction).TxId
			}).
			Return(newEvaluateResponse(nil), nil).
			Times(1)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

		proposal, err := contract.NewProposal("transaction")
		require.NoError(t, err, "NewProposal")
		_, err = proposal.Evaluate()
		require.NoError(t, err, "Evaluate")

		require.Equal(t, proposal.TransactionID(), actual)
	})

	t.Run("Includes transaction ID in evaluate request", func(t *testing.T) {
		var actual string
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, in *gateway.EvaluateRequest, _ ...grpc.CallOption) {
				actual = in.TransactionId
			}).
			Return(newEvaluateResponse(nil), nil).
			Times(1)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

		proposal, err := contract.NewProposal("transaction")
		require.NoError(t, err, "NewProposal")
		_, err = proposal.Evaluate()
		require.NoError(t, err, "Evaluate")

		require.Equal(t, proposal.TransactionID(), actual)
	})

	t.Run("Uses sign", func(t *testing.T) {
		var actual []byte
		expected := []byte("MY_SIGNATURE")
		sign := func(digest []byte) ([]byte, error) {
			return expected, nil
		}
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, in *gateway.EvaluateRequest, _ ...grpc.CallOption) {
				actual = in.ProposedTransaction.Signature
			}).
			Return(newEvaluateResponse(nil), nil).
			Times(1)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient), WithSign(sign))

		_, err := contract.EvaluateTransaction("transaction")
		require.NoError(t, err)

		require.EqualValues(t, expected, actual)
	})

	t.Run("Uses hash", func(t *testing.T) {
		var actual []byte
		expected := []byte("MY_DIGEST")
		sign := func(digest []byte) ([]byte, error) {
			actual = digest
			return expected, nil
		}
		hash := func(message []byte) []byte {
			return expected
		}
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Return(newEvaluateResponse(nil), nil)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient), WithSign(sign), WithHash(hash))

		_, err := contract.EvaluateTransaction("transaction")
		require.NoError(t, err)

		require.EqualValues(t, expected, actual)
	})

	t.Run("Sends private data with evaluate", func(t *testing.T) {
		var actualOrgs []string
		expectedOrgs := []string{"MY_ORG"}
		var actualPrice []byte
		expectedPrice := []byte("3000")
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			Do(func(_ context.Context, in *gateway.EvaluateRequest, _ ...grpc.CallOption) {
				actualOrgs = in.TargetOrganizations
				transient := test.AssertUnmarshalProposalPayload(t, in.ProposedTransaction).TransientMap
				actualPrice = transient["price"]
			}).
			Return(newEvaluateResponse(nil), nil)

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

		privateData := map[string][]byte{
			"price": []byte("3000"),
		}

		_, err := contract.Evaluate("transaction", WithTransient(privateData), WithEndorsingOrganizations("MY_ORG"))
		require.NoError(t, err)

		require.EqualValues(t, expectedOrgs, actualOrgs)
		require.EqualValues(t, expectedPrice, actualPrice)
	})

	for name, testCase := range map[string]struct {
		run func(*testing.T, context.Context, *Contract) ([]byte, error)
	}{
		"Proposal uses specified context": {
			run: func(t *testing.T, ctx context.Context, contract *Contract) ([]byte, error) {
				proposal, err := contract.NewProposal("transaction")
				require.NoError(t, err, "NewProposal")
				return proposal.EvaluateWithContext(ctx)
			},
		},
		"Contract uses specified context": {
			run: func(t *testing.T, ctx context.Context, contract *Contract) ([]byte, error) {
				return contract.EvaluateWithContext(ctx, "transaction")
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			mockClient := NewMockGatewayClient(gomock.NewController(t))
			mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, _ *gateway.EvaluateRequest, _ ...grpc.CallOption) (*gateway.EvaluateResponse, error) {
					err := ctx.Err()
					if err != nil {
						return nil, err
					}
					return newEvaluateResponse(nil), nil
				})

			contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

			_, err := testCase.run(t, ctx, contract)

			require.ErrorIs(t, err, context.Canceled)
		})
	}

	t.Run("Uses default context", func(t *testing.T) {
		mockClient := NewMockGatewayClient(gomock.NewController(t))
		mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, _ *gateway.EvaluateRequest, _ ...grpc.CallOption) (*gateway.EvaluateResponse, error) {
				select {
				case <-time.After(1 * time.Second):
					return newEvaluateResponse(nil), nil
				case <-ctx.Done(): // Zero timeout context should cancel immediately, selecting this case
					return nil, ctx.Err()
				}
			})

		contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient), WithEvaluateTimeout(0))

		_, err := contract.Evaluate("transaction")

		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	for testName, testCase := range map[string]struct {
		run func(*testing.T, *Contract, []grpc.CallOption) ([]byte, error)
	}{
		"Uses specified gRPC call options": {
			run: func(t *testing.T, contract *Contract, expected []grpc.CallOption) ([]byte, error) {
				proposal, err := contract.NewProposal("transaction")
				require.NoError(t, err, "NewProposal")
				return proposal.Evaluate(expected...)
			},
		},
		"Uses specified gRPC call options with specified context": {
			run: func(t *testing.T, contract *Contract, expected []grpc.CallOption) ([]byte, error) {
				proposal, err := contract.NewProposal("transaction")
				require.NoError(t, err, "NewProposal")
				return proposal.EvaluateWithContext(context.Background(), expected...)
			},
		},
	} {
		t.Run(testName, func(t *testing.T) {
			var actual []grpc.CallOption
			expected := grpc.WaitForReady(true)

			mockClient := NewMockGatewayClient(gomock.NewController(t))
			mockClient.EXPECT().Evaluate(gomock.Any(), gomock.Any(), gomock.Any()).
				Do(func(_ context.Context, _ *gateway.EvaluateRequest, opts ...grpc.CallOption) {
					actual = opts
				}).
				Return(newEvaluateResponse(nil), nil).
				Times(1)

			contract := AssertNewTestContract(t, "chaincode", WithGatewayClient(mockClient))

			_, err := testCase.run(t, contract, []grpc.CallOption{expected})
			require.NoError(t, err, "Evaluate")

			require.Contains(t, actual, expected, "CallOptions")
		})
	}
}
