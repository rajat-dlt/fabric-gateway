/*
 * Copyright 2019 IBM All Rights Reserved.
 *
 * SPDX-License-Identifier: Apache-2.0
 */

package org.hyperledger.fabric.client.identity;

import org.bouncycastle.asn1.x500.X500Name;
import org.bouncycastle.asn1.x509.AlgorithmIdentifier;
import org.bouncycastle.asn1.x509.SubjectPublicKeyInfo;
import org.bouncycastle.cert.X509CertificateHolder;
import org.bouncycastle.cert.X509v3CertificateBuilder;
import org.bouncycastle.cert.jcajce.JcaX509CertificateConverter;
import org.bouncycastle.crypto.params.AsymmetricKeyParameter;
import org.bouncycastle.crypto.util.PrivateKeyFactory;
import org.bouncycastle.jce.provider.BouncyCastleProvider;
import org.bouncycastle.operator.ContentSigner;
import org.bouncycastle.operator.DefaultDigestAlgorithmIdentifierFinder;
import org.bouncycastle.operator.DefaultSignatureAlgorithmIdentifierFinder;
import org.bouncycastle.operator.OperatorCreationException;
import org.bouncycastle.operator.bc.BcECContentSignerBuilder;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.math.BigInteger;
import java.security.InvalidAlgorithmParameterException;
import java.security.KeyPair;
import java.security.KeyPairGenerator;
import java.security.NoSuchAlgorithmException;
import java.security.PrivateKey;
import java.security.Provider;
import java.security.cert.CertificateException;
import java.security.cert.X509Certificate;
import java.security.spec.AlgorithmParameterSpec;
import java.security.spec.ECGenParameterSpec;
import java.util.Date;
import java.util.Locale;

public final class X509Credentials {
    public enum Curve {
        P256(() -> {
            KeyPairGenerator generator = KeyPairGenerator.getInstance("EC", BC_PROVIDER);
            generator.initialize(new ECGenParameterSpec("P-256"));
            return generator;
        }),
        P384(() -> {
            KeyPairGenerator generator = KeyPairGenerator.getInstance("EC", BC_PROVIDER);
            generator.initialize(new ECGenParameterSpec("P-384"));
            return generator;
        }),
        Ed25519(() -> KeyPairGenerator.getInstance("Ed25519", BC_PROVIDER));

        @FunctionalInterface
        private interface KeyPairGeneratorSupplier {
            KeyPairGenerator get() throws NoSuchAlgorithmException, InvalidAlgorithmParameterException;
        }

        private final KeyPairGeneratorSupplier generatorSupplier;

        Curve(final KeyPairGeneratorSupplier generatorSupplier) {
            this.generatorSupplier = generatorSupplier;
        }

        public KeyPair generateKeyPair() {
            try {
                return generatorSupplier.get().generateKeyPair();
            } catch (NoSuchAlgorithmException | InvalidAlgorithmParameterException e) {
                throw new RuntimeException(e);
            }
        }
    }

    private static final Provider BC_PROVIDER = new BouncyCastleProvider();

    private final X509Certificate certificate;
    private final PrivateKey privateKey;

    /**
     * Create credentials using a P-256 curve.
     */
    public X509Credentials() {
        this(Curve.P256);
    }

    public X509Credentials(Curve curve) {
        KeyPair keyPair = curve.generateKeyPair();
        certificate = generateCertificate(keyPair);
        privateKey = keyPair.getPrivate();
    }

    private X509Certificate generateCertificate(KeyPair keyPair) {
        X500Name dnName = new X500Name("CN=John Doe");
        Date validityBeginDate = new Date(System.currentTimeMillis() - 24L * 60 * 60 * 1000); // Yesterday
        Date validityEndDate = new Date(System.currentTimeMillis() + 2L * 365 * 24 * 60 * 60 * 1000); // 2 years from now
        SubjectPublicKeyInfo subPubKeyInfo = SubjectPublicKeyInfo.getInstance(keyPair.getPublic().getEncoded());
        X509v3CertificateBuilder builder = new X509v3CertificateBuilder(
                dnName,
                BigInteger.valueOf(System.currentTimeMillis()),
                validityBeginDate,
                validityEndDate,
                Locale.getDefault(),
                dnName,
                subPubKeyInfo);

        AlgorithmIdentifier sigAlgId = new DefaultSignatureAlgorithmIdentifierFinder().find("SHA256WithRSAEncryption");
        AlgorithmIdentifier digAlgId = new DefaultDigestAlgorithmIdentifierFinder().find(sigAlgId);

        try {
            KeyPair signerKeys = Curve.P256.generateKeyPair();
            AsymmetricKeyParameter keyParameter = PrivateKeyFactory.createKey(signerKeys.getPrivate().getEncoded());
            ContentSigner contentSigner = new BcECContentSignerBuilder(sigAlgId, digAlgId)
                    .build(keyParameter);
            X509CertificateHolder holder = builder.build(contentSigner);
            return new JcaX509CertificateConverter().getCertificate(holder);
        } catch (IOException e) {
            throw new UncheckedIOException(e);
        } catch (OperatorCreationException | CertificateException e) {
            throw new RuntimeException(e);
        }
    }

    public X509Certificate getCertificate() {
        return certificate;
    }

    public String getCertificatePem() {
        return Identities.toPemString(certificate);
    }

    public PrivateKey getPrivateKey() {
        return privateKey;
    }

    public String getPrivateKeyPem() {
        return Identities.toPemString(privateKey);
    }
}
